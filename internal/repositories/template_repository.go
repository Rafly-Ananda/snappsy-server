package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type TemplateRepository interface {
	EnsureIndexes(ctx context.Context) error
	InsertTemplate(ctx context.Context, template models.Template) (string, error)
	UpdateTemplate(ctx context.Context, template models.Template) error
	FindTemplateByID(ctx context.Context, id string) (models.Template, error)
	ListTemplates(ctx context.Context) ([]models.Template, error)
	ListTemplatesByEvent(ctx context.Context, eventID string, kind string) ([]models.Template, error)
	InsertVersion(ctx context.Context, version models.TemplateVersion) (string, error)
	UpdateVersion(ctx context.Context, version models.TemplateVersion) error
	FindVersion(ctx context.Context, templateID string, version int) (models.TemplateVersion, error)
	ListVersions(ctx context.Context, templateID string) ([]models.TemplateVersion, error)
	DeleteTemplate(ctx context.Context, templateID string) error
	DeleteVersions(ctx context.Context, templateID string) error
}
