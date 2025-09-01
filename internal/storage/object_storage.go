package storage

import (
	"context"
	"time"
)

type ObjectStorage interface {
	PresignPut(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error)
	PresignGet(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, bucket string, key string) error
	Exists(ctx context.Context, bucket string, key string) (bool, error)
}
