package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type EventRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, event models.Event) (string, error)
	Update(ctx context.Context, event models.Event) error
	FindByID(ctx context.Context, id string) (models.Event, error)
	FindBySlug(ctx context.Context, slug string) (models.Event, error)
	FindByDisplayTokenHash(ctx context.Context, hash string) (models.Event, error)
	List(ctx context.Context) ([]models.Event, error)
	ListByOwner(ctx context.Context, ownerType models.EventOwnerType, ownerID string) ([]models.Event, error)
	ConsumeQuota(ctx context.Context, id string, kind models.MediaKind, allowOverQuota bool) (bool, error)
	ReleaseQuota(ctx context.Context, id string, kind models.MediaKind) error
	ClearTemplateReference(ctx context.Context, templateID string) error
	UpdateTemplateVersion(ctx context.Context, templateID string, version int) error
}
