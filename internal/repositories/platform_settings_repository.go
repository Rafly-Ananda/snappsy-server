package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type PlatformSettingsRepository interface {
	EnsureIndexes(ctx context.Context) error
	Find(ctx context.Context, key string) (models.PlatformSettings, bool, error)
	Upsert(ctx context.Context, settings models.PlatformSettings) error
}
