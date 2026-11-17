package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
	"go.mongodb.org/mongo-driver/mongo"
)

type BoomerangWorker struct {
	jobs      repositories.MediaJobRepository
	media     repositories.MediaRepository
	obj       storage.ObjectStorage
	cfg       config.BoomerangConfig
	bucket    string
	drive     DriveEnqueuer
	cleaner   DriveCleaner
	events    repositories.EventRepository
	templates repositories.TemplateRepository
	assets    repositories.AssetRepository
	hub       *websocket.Hub
}

func (w *BoomerangWorker) SetEventRepository(events repositories.EventRepository) {
	w.events = events
}

func (w *BoomerangWorker) SetHub(hub *websocket.Hub) {
	w.hub = hub
}

func (w *BoomerangWorker) SetTemplateRepository(templates repositories.TemplateRepository) {
	w.templates = templates
}

func (w *BoomerangWorker) SetAssetRepository(assets repositories.AssetRepository) {
	w.assets = assets
}

func (w *BoomerangWorker) videoOverlay(ctx context.Context, eventID, dir string) string {
	if w.events == nil || w.templates == nil || w.assets == nil {
		return ""
	}
	event, err := w.events.FindByID(ctx, eventID)
	if err != nil || event.BoomerangTemplateID == "" {
		return ""
	}
	version, err := w.templates.FindVersion(ctx, event.BoomerangTemplateID, event.BoomerangTemplateVersion)
	if err != nil || version.Status != "published" {
		return ""
	}
	for _, layer := range version.Schema.Layers {
		if layer.Type != models.TemplateLayerImage || layer.AssetID == "" {
			continue
		}
		asset, err := w.assets.FindByID(ctx, layer.AssetID)
		if err != nil {
			continue
		}
		reader, err := w.obj.Get(ctx, w.bucket, asset.ObjectKey)
		if err != nil {
			continue
		}
		overlayPath := filepath.Join(dir, "overlay.png")
		file, createErr := os.Create(overlayPath)
		if createErr != nil {
			reader.Close()
			continue
		}
		_, copyErr := io.Copy(file, reader)
		file.Close()
		reader.Close()
		if copyErr == nil {
			return overlayPath
		}
	}
	return ""
}

func NewBoomerangWorker(jobs repositories.MediaJobRepository, media repositories.MediaRepository, obj storage.ObjectStorage, cfg config.BoomerangConfig, bucket string, drive ...DriveEnqueuer) *BoomerangWorker {
	worker := &BoomerangWorker{jobs: jobs, media: media, obj: obj, cfg: cfg, bucket: bucket}
	if len(drive) > 0 {
		worker.drive = drive[0]
		if cleaner, ok := drive[0].(DriveCleaner); ok {
			worker.cleaner = cleaner
		}
	}
	return worker
}

func (w *BoomerangWorker) Run(ctx context.Context) {
	if _, err := exec.LookPath(w.cfg.FFmpegPath); err != nil {
		log.Printf("boomerang worker: ffmpeg unavailable: %v", err)
		return
	}
	if _, err := exec.LookPath(w.cfg.FFprobePath); err != nil {
		log.Printf("boomerang worker: ffprobe unavailable: %v", err)
		return
	}
	go w.cleanupLoop(ctx)
	for i := 0; i < w.cfg.Concurrency; i++ {
		go w.loop(ctx, i)
	}
	<-ctx.Done()
}

func (w *BoomerangWorker) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	w.cleanup(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanup(ctx)
		}
	}
}

func (w *BoomerangWorker) cleanup(ctx context.Context) {
	items, err := w.media.FindExpired(ctx, time.Now(), 50)
	if err != nil {
		log.Printf("boomerang worker: retention query failed: %v", err)
		return
	}
	for _, item := range items {
		keys := []string{item.ObjectKey, item.BaseKey, item.CompositeKey, item.ThumbnailKey, item.SourceKey, item.OutputKey, item.PosterKey}
		seen := make(map[string]bool)
		for _, key := range keys {
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			if err := w.obj.Delete(ctx, w.bucket, key); err != nil {
				log.Printf("boomerang worker: retention delete %q failed: %v", key, err)
				keys = nil
				break
			}
		}
		if keys == nil {
			continue
		}
		if w.cleaner != nil {
			if err := w.cleaner.DeleteMediaFiles(ctx, item.EventID, item.ID.Hex()); err != nil {
				log.Printf("boomerang worker: retention drive files for %s failed: %v", item.ID.Hex(), err)
				continue
			}
		}
		if err := w.jobs.DeleteByMediaID(ctx, item.ID.Hex()); err != nil {
			log.Printf("boomerang worker: retention jobs for %s failed: %v", item.ID.Hex(), err)
			continue
		}
		if err := w.media.Delete(ctx, item.ID.Hex()); err != nil {
			log.Printf("boomerang worker: retention media %s failed: %v", item.ID.Hex(), err)
		}
	}
}

func (w *BoomerangWorker) loop(ctx context.Context, index int) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		job, err := w.jobs.ClaimNext(ctx)
		if errors.Is(err, mongo.ErrNoDocuments) {
			continue
		}
		if err != nil {
			if ctx.Err() == nil {
				log.Printf("boomerang worker %d: claim failed: %v", index, err)
			}
			continue
		}
		w.process(ctx, job)
	}
}

func (w *BoomerangWorker) process(ctx context.Context, job models.MediaJob) {
	dir, err := os.MkdirTemp("", "snappsy-boomerang-*")
	if err != nil {
		w.fail(ctx, job, "temp_dir_failed")
		return
	}
	defer os.RemoveAll(dir)

	sourcePath := filepath.Join(dir, "source"+path.Ext(job.SourceKey))
	if err := w.downloadSource(ctx, job.SourceKey, sourcePath); err != nil {
		w.fail(ctx, job, "source_download_failed")
		return
	}
	info, err := os.Stat(sourcePath)
	if err != nil || info.Size() <= 0 || (w.cfg.MaxSourceBytes > 0 && info.Size() > w.cfg.MaxSourceBytes) {
		w.fail(ctx, job, "source_too_large")
		return
	}
	duration, err := w.probeDuration(ctx, sourcePath)
	if err != nil || duration <= 0 || duration > float64(w.cfg.MaxSourceSeconds) {
		w.fail(ctx, job, "source_invalid")
		return
	}
	segment := job.SegmentSeconds
	if segment <= 0 || segment > float64(w.cfg.SegmentSeconds) {
		segment = float64(w.cfg.SegmentSeconds)
	}
	start := job.StartSeconds
	if start < 0 {
		start = 0
	}
	if start+segment > duration {
		start = duration - segment
	}
	if start < 0 {
		start = 0
	}
	if duration < 0.5 {
		w.fail(ctx, job, "source_too_short")
		return
	}
	item, err := w.media.FindByID(ctx, job.MediaID)
	if err != nil {
		w.fail(ctx, job, "media_missing")
		return
	}
	style := w.videoStyle(ctx, item.EventID)
	overlayPath := w.videoOverlay(ctx, item.EventID, dir)

	outputPath := filepath.Join(dir, "loop.mp4")
	posterPath := filepath.Join(dir, "poster.jpg")
	thumbnailPath := filepath.Join(dir, "thumbnail.jpg")
	args := []string{"-y", "-ss", formatSeconds(start), "-t", formatSeconds(segment), "-i", sourcePath}
	filter := w.buildVideoFilter(dir, item, style)
	if overlayPath != "" {
		args = append(args, "-loop", "1", "-i", overlayPath)
		filter = filter + "[bg];[1:v]scale=720:1280[ov];[bg][ov]overlay=0:0"
	}
	filterFlag := "-vf"
	if overlayPath != "" {
		filterFlag = "-filter_complex"
	}
	args = append(args, "-an", filterFlag, filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
		"-movflags", "+faststart", outputPath)
	if err := w.runFFmpeg(ctx, args...); err != nil {
		w.fail(ctx, job, "transcode_failed")
		return
	}
	if err := w.runFFmpeg(ctx, "-y", "-i", outputPath, "-frames:v", "1", "-vf", "scale=720:-2", posterPath); err != nil {
		w.fail(ctx, job, "poster_failed")
		return
	}
	if err := w.runFFmpeg(ctx, "-y", "-i", posterPath, "-vf", "scale=360:-2", thumbnailPath); err != nil {
		w.fail(ctx, job, "thumbnail_failed")
		return
	}
	baseKey := path.Dir(job.SourceKey)
	outputKey := path.Join(baseKey, "loop.mp4")
	posterKey := path.Join(baseKey, "poster.jpg")
	thumbnailKey := path.Join(baseKey, "thumbnail.jpg")
	if err := w.uploadFile(ctx, outputKey, outputPath, "video/mp4"); err != nil {
		w.fail(ctx, job, "output_upload_failed")
		return
	}
	if err := w.uploadFile(ctx, posterKey, posterPath, "image/jpeg"); err != nil {
		w.fail(ctx, job, "poster_upload_failed")
		return
	}
	if err := w.uploadFile(ctx, thumbnailKey, thumbnailPath, "image/jpeg"); err != nil {
		w.fail(ctx, job, "thumbnail_upload_failed")
		return
	}

	item.ObjectKey = outputKey
	item.OutputKey = outputKey
	item.PosterKey = posterKey
	item.ThumbnailKey = thumbnailKey
	item.ProcessingState = models.MediaProcessingReady
	item.UpdatedAt = time.Now()
	if err := w.media.Update(ctx, item); err != nil {
		w.fail(ctx, job, "media_update_failed")
		return
	}
	if err := w.jobs.Complete(ctx, job.ID.Hex()); err != nil {
		log.Printf("boomerang worker: complete job %s: %v", job.ID.Hex(), err)
	}
	if w.drive != nil {
		if err := w.drive.Enqueue(ctx, item); err != nil {
			log.Printf("drive enqueue boomerang %s: %v", item.ID.Hex(), err)
		}
	}
	if w.hub != nil {
		w.hub.Publish(item.EventID, websocket.MediaReadyMessage(item.EventID, item.ID.Hex(), string(item.Kind)))
	}
}

func (w *BoomerangWorker) fail(ctx context.Context, job models.MediaJob, code string) {
	if err := w.jobs.Fail(ctx, job.ID.Hex(), code); err != nil {
		log.Printf("boomerang worker: fail job %s: %v", job.ID.Hex(), err)
	}
	item, err := w.media.FindByID(ctx, job.MediaID)
	if err == nil {
		item.ProcessingState = models.MediaProcessingFailed
		item.UpdatedAt = time.Now()
		_ = w.media.Update(ctx, item)
		if w.events != nil {
			_ = w.events.ReleaseQuota(ctx, item.EventID, models.MediaKindBoomerang)
		}
	}
}

func (w *BoomerangWorker) downloadSource(ctx context.Context, key string, destination string) error {
	reader, err := w.obj.Get(ctx, w.bucket, key)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, io.LimitReader(reader, w.cfg.MaxSourceBytes+1))
	return err
}

func (w *BoomerangWorker) uploadFile(ctx context.Context, key string, filePath string, contentType string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	return w.obj.Put(ctx, w.bucket, key, file, info.Size(), contentType)
}

func (w *BoomerangWorker) probeDuration(ctx context.Context, filePath string) (float64, error) {
	cmd := exec.CommandContext(ctx, w.cfg.FFprobePath, "-v", "error", "-show_entries", "format=duration", "-of", "default=nw=1:nk=1", filePath)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func (w *BoomerangWorker) runFFmpeg(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, w.cfg.FFmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w: %s", err, tail(string(out), 400))
	}
	return nil
}

func (w *BoomerangWorker) videoStyle(ctx context.Context, eventID string) models.TemplateStyle {
	style := models.TemplateStyle{Paper: "#F2E8D5", Ink: "#191714", Accent: "#ED7648"}
	if w.events == nil || w.templates == nil {
		return style
	}
	event, err := w.events.FindByID(ctx, eventID)
	if err != nil || event.BoomerangTemplateID == "" {
		return style
	}
	version, err := w.templates.FindVersion(ctx, event.BoomerangTemplateID, event.BoomerangTemplateVersion)
	if err != nil || version.Status != "published" {
		return style
	}
	if version.Schema.Style.Paper != "" {
		style = version.Schema.Style
	}
	return style
}

func (w *BoomerangWorker) buildVideoFilter(dir string, item models.MediaItem, style models.TemplateStyle) string {
	paper := filterColor(style.Paper, "0xF2E8D5")
	ink := filterColor(style.Ink, "0x191714")
	accent := filterColor(style.Accent, "0xED7648")
	parts := []string{
		"split[a][b]",
		"[b]reverse[r]",
		"[a][r]concat=n=2:v=1:a=0",
		"scale=640:1040:force_original_aspect_ratio=decrease",
		"pad=640:1040:(ow-iw)/2:(oh-ih)/2:color=" + paper,
		"pad=720:1280:40:40:color=" + accent,
		"drawbox=x=40:y=40:w=640:h=1040:color=" + ink + ":t=6",
		"drawbox=x=0:y=1080:w=720:h=200:color=" + paper + ":t=fill",
		"drawbox=x=0:y=1080:w=720:h=6:color=" + ink + ":t=fill",
	}
	font := videoFontPath()
	captionFont := videoCaptionFontPath()
	if font != "" {
		parts = append(parts, "drawtext=fontfile="+font+":text='snappsy':fontcolor="+ink+":fontsize=36:x=(w-text_w)/2:y=1105")
		namePath := filepath.Join(dir, "name.txt")
		if err := os.WriteFile(namePath, []byte(strings.TrimSpace(item.Username)), 0o600); err == nil && strings.TrimSpace(item.Username) != "" {
			parts = append(parts, "drawtext=fontfile="+font+":textfile="+namePath+":fontcolor="+ink+":fontsize=28:x=40:y=1168")
		}
		captionPath := filepath.Join(dir, "caption.txt")
		if captionFont != "" {
			font = captionFont
		}
		if err := os.WriteFile(captionPath, []byte(strings.TrimSpace(item.Captions)), 0o600); err == nil && strings.TrimSpace(item.Captions) != "" {
			parts = append(parts, "drawtext=fontfile="+font+":textfile="+captionPath+":fontcolor="+ink+":fontsize=26:x=40:y=1210")
		}
	}
	parts = append(parts, "fps=30")
	return strings.Join(parts, ",")
}

func filterColor(value, fallback string) string {
	if len(value) == 7 && value[0] == '#' {
		return "0x" + strings.ToUpper(value[1:])
	}
	return fallback
}

func videoFontPath() string {
	candidates := []string{
		"/app/assets/fonts/Caprasimo-Regular.ttf",
		"assets/fonts/Caprasimo-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func videoCaptionFontPath() string {
	candidates := []string{
		"/app/assets/fonts/Caveat-Variable.ttf",
		"assets/fonts/Caveat-Variable.ttf",
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

func formatSeconds(value float64) string {
	return strconv.FormatFloat(value, 'f', 3, 64)
}

func tail(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
