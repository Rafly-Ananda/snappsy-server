package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type ImageRepository interface {
	Insert(ctx context.Context, image models.Images) (string, error)
	FindAllByEvents(ctx context.Context, eventId string, cursor string, limit int) ([]models.Images, string, error)
}
