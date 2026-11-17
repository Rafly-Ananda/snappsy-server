package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type AssetRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, asset models.Asset) (string, error)
	List(ctx context.Context, kind string) ([]models.Asset, error)
	ListPlatform(ctx context.Context, kind string) ([]models.Asset, error)
	ListByEvent(ctx context.Context, eventID string, kind string) ([]models.Asset, error)
	FindByID(ctx context.Context, id string) (models.Asset, error)
	Update(ctx context.Context, asset models.Asset) error
}
