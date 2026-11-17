package services

import (
	"context"
	"log"
	"strings"
	"time"

	imgDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/images"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
)

type ImageService struct {
	repo         repositories.MediaRepository
	events       repositories.EventRepository
	obj          storage.ObjectStorage
	bucket       string
	presignedTtl time.Duration
}

func NewImageService(
	repo repositories.MediaRepository,
	events repositories.EventRepository,
	obj storage.ObjectStorage,
	bucket string,
	presignedTtl time.Duration,
) *ImageService {
	return &ImageService{repo: repo, events: events, obj: obj, bucket: bucket, presignedTtl: presignedTtl}
}

func (s *ImageService) GetAllPresignedImagesByEvent(ctx context.Context, eventId string, cursor string, limit int, expiry time.Duration) ([]imgDto.GeneratePresignedUrlView, string, error) {
	event, err := s.events.FindByID(ctx, eventId)
	if err != nil {
		return []imgDto.GeneratePresignedUrlView{}, "", ErrEventNotFound
	}
	imgs, next, err := s.repo.FindAllByEvents(ctx, eventId, "", cursor, limit)
	if err != nil {
		return []imgDto.GeneratePresignedUrlView{}, "", err
	}

	presignedList := make([]imgDto.GeneratePresignedUrlView, 0, len(imgs))
	for _, item := range imgs {
		if item.ProcessingState != "" && item.ProcessingState != models.MediaProcessingReady {
			continue
		}
		if item.ModerationState == models.MediaModerationHidden {
			continue
		}
		if event.UploadPolicy.RequireApproval && item.ModerationState != models.MediaModerationApproved {
			continue
		}
		key := item.ThumbnailKey
		if key == "" {
			key = item.ObjectKey
		}
		if key == "" {
			continue
		}
		url, err := s.obj.PresignGet(ctx, s.bucket, key, expiry)
		if err != nil {
			log.Printf("presign failed for key %s: %v", item.ObjectKey, err)
			continue
		}
		view := imgDto.GeneratePresignedUrlView{
			Id:          item.ID.Hex(),
			Kind:        string(item.Kind),
			Url:         url,
			Captions:    item.Captions,
			From:        item.Username,
			Orientation: item.Orientation,
			CreatedAt:   item.CreatedAt.Format(time.RFC3339),
		}
		if item.Kind == models.MediaKindBoomerang {
			playbackKey := item.OutputKey
			if playbackKey == "" {
				playbackKey = item.ObjectKey
			}
			if playbackKey != "" {
				view.PlaybackUrl, _ = s.obj.PresignGet(ctx, s.bucket, playbackKey, expiry)
			}
		}
		presignedList = append(presignedList, view)
	}

	return presignedList, next, nil
}

func safeObjectKeyPart(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "unknown"
	}
	return b.String()
}
