package services

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type workerJobRepo struct {
	completed    string
	failed       string
	deletedMedia string
}

func (f *workerJobRepo) EnsureIndexes(ctx context.Context) error               { return nil }
func (f *workerJobRepo) Insert(ctx context.Context, job models.MediaJob) error { return nil }
func (f *workerJobRepo) ClaimNext(ctx context.Context) (models.MediaJob, error) {
	return models.MediaJob{}, nil
}
func (f *workerJobRepo) CountActiveByEvent(ctx context.Context, eventID string) (int64, error) {
	return 0, nil
}
func (f *workerJobRepo) SummaryByEvent(ctx context.Context, eventID string) (int64, int64, error) {
	return 0, 0, nil
}
func (f *workerJobRepo) Complete(ctx context.Context, id string) error {
	f.completed = id
	return nil
}
func (f *workerJobRepo) Fail(ctx context.Context, id string, code string) error {
	f.failed = id
	return nil
}
func (f *workerJobRepo) DeleteByMediaID(ctx context.Context, mediaID string) error {
	f.deletedMedia = mediaID
	return nil
}

func TestBoomerangWorkerCleanupRemovesExpiredMedia(t *testing.T) {
	mediaID := primitive.NewObjectID()
	item := models.MediaItem{
		ID:           mediaID,
		EventID:      "event-1",
		Kind:         models.MediaKindBoomerang,
		ObjectKey:    "events/event-1/loop.mp4",
		SourceKey:    "events/event-1/source.mp4",
		OutputKey:    "events/event-1/loop.mp4",
		PosterKey:    "events/event-1/poster.jpg",
		ThumbnailKey: "events/event-1/thumbnail.jpg",
		RetentionAt:  func() *time.Time { t := time.Now().Add(-time.Hour); return &t }(),
	}
	media := &fakePhotoMediaRepo{expired: []models.MediaItem{item}, byID: map[string]models.MediaItem{mediaID.Hex(): item}}
	jobs := &workerJobRepo{}
	obj := &fakeObjectStorage{exists: true}
	worker := NewBoomerangWorker(jobs, media, obj, config.BoomerangConfig{}, "images")

	worker.cleanup(context.Background())

	if len(obj.deleted) != 4 {
		t.Fatalf("cleanup deleted %d objects: %v", len(obj.deleted), obj.deleted)
	}
	if jobs.deletedMedia != mediaID.Hex() {
		t.Fatalf("cleanup deleted jobs for %q", jobs.deletedMedia)
	}
	if _, ok := media.byID[mediaID.Hex()]; ok {
		t.Fatal("cleanup did not delete media record")
	}
}

func TestVideoFilterIncludesFrame(t *testing.T) {
	worker := &BoomerangWorker{}
	filter := worker.buildVideoFilter(t.TempDir(), models.MediaItem{Username: "Ada", Captions: "hello"}, models.TemplateStyle{})
	for _, want := range []string{"pad=720:1280:40:40:color=0xED7648", "drawbox=x=0:y=1080:w=720:h=200:color=0xF2E8D5:t=fill", "fps=30"} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter missing %q: %s", want, filter)
		}
	}
}

func TestBoomerangWorkerWritesReadyMedia(t *testing.T) {
	dir := t.TempDir()
	ffmpegPath := filepath.Join(dir, "ffmpeg")
	ffprobePath := filepath.Join(dir, "ffprobe")
	ffmpegScript := "#!/bin/sh\nfor last; do :; done\nprintf video > \"$last\"\n"
	ffprobeScript := "#!/bin/sh\necho 3.0\n"
	if err := os.WriteFile(ffmpegPath, []byte(ffmpegScript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ffprobePath, []byte(ffprobeScript), 0o755); err != nil {
		t.Fatal(err)
	}

	mediaID := primitive.NewObjectID()
	item := models.MediaItem{ID: mediaID, EventID: "event-1", Kind: models.MediaKindBoomerang, ProcessingState: models.MediaProcessingProcessing}
	media := &fakePhotoMediaRepo{byID: map[string]models.MediaItem{mediaID.Hex(): item}}
	jobs := &workerJobRepo{}
	obj := &fakeObjectStorage{exists: true}
	worker := NewBoomerangWorker(jobs, media, obj, config.BoomerangConfig{
		FFmpegPath:       ffmpegPath,
		FFprobePath:      ffprobePath,
		MaxSourceBytes:   1024,
		MaxSourceSeconds: 30,
		SegmentSeconds:   3,
	}, "images")

	job := models.MediaJob{
		ID:             primitive.NewObjectID(),
		MediaID:        mediaID.Hex(),
		EventID:        "event-1",
		SourceKey:      "events/event-1/boomerangs/source.mp4",
		State:          models.MediaJobProcessing,
		SegmentSeconds: 3,
		CreatedAt:      time.Now(),
	}
	worker.process(context.Background(), job)

	saved := media.byID[mediaID.Hex()]
	if saved.ProcessingState != models.MediaProcessingReady || saved.OutputKey == "" || saved.PosterKey == "" || saved.ThumbnailKey == "" || saved.PosterKey == saved.ThumbnailKey {
		t.Fatalf("worker media = %+v", saved)
	}
	if jobs.completed != job.ID.Hex() {
		t.Fatalf("worker completed job = %q, want %q", jobs.completed, job.ID.Hex())
	}
}
