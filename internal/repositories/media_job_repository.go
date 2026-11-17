package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type MediaJobRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, job models.MediaJob) error
	ClaimNext(ctx context.Context) (models.MediaJob, error)
	CountActiveByEvent(ctx context.Context, eventID string) (int64, error)
	SummaryByEvent(ctx context.Context, eventID string) (active int64, failed int64, err error)
	Complete(ctx context.Context, id string) error
	Fail(ctx context.Context, id string, code string) error
	DeleteByMediaID(ctx context.Context, mediaID string) error
}
