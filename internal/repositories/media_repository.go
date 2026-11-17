package repositories

import (
	"context"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type MediaRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, item models.MediaItem) (string, error)
	Update(ctx context.Context, item models.MediaItem) error
	FindByID(ctx context.Context, id string) (models.MediaItem, error)
	FindByUploadIntentID(ctx context.Context, intentID string) (models.MediaItem, error)
	FindAllByEvents(ctx context.Context, eventId string, moderationState string, after string, limit int) ([]models.MediaItem, string, error)
	FindExpired(ctx context.Context, now time.Time, limit int) ([]models.MediaItem, error)
	FindReadyByEvent(ctx context.Context, eventId string) ([]models.MediaItem, error)
	Delete(ctx context.Context, id string) error
}
