package storage

import (
	"context"
	"io"
	"time"
)

type ObjectStorage interface {
	PresignPut(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error)
	PresignGet(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, bucket string, key string) error
	Exists(ctx context.Context, bucket string, key string) (bool, error)
	Get(ctx context.Context, bucket string, key string) (io.ReadCloser, error)
	Put(ctx context.Context, bucket string, key string, body io.Reader, size int64, contentType string) error
}
