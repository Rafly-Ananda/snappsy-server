package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type AdminSessionRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, session models.AdminSession) error
	FindByTokenHash(ctx context.Context, tokenHash string) (models.AdminSession, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
}
