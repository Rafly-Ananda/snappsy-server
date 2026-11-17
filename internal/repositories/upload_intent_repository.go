package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type UploadIntentRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, intent models.UploadIntent) (string, error)
	FindByID(ctx context.Context, id string) (models.UploadIntent, error)
	Complete(ctx context.Context, id string, mediaID string) error
}
