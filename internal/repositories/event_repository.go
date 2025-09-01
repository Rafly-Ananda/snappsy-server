package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type EventRepository interface {
	Insert(ctx context.Context, even models.Events) (string, error)
}
