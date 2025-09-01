package storage

import (
	"context"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioAdapter struct {
	Client *minio.Client
	Bucket string
}

func NewMinio(endpoint, accessKey, secretKey, bucket string, minioExpiry time.Duration, secure bool) (*MinioAdapter, error) {
	cl, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})

	if err != nil {
		return nil, err
	}

	return &MinioAdapter{
		Client: cl,
		Bucket: bucket}, nil
}

func (m *MinioAdapter) PresignPut(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	u, err := m.Client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *MinioAdapter) PresignGet(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	u, err := m.Client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (m *MinioAdapter) Delete(ctx context.Context, bucket, key string) error {
	return m.Client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

func (m *MinioAdapter) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := m.Client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		// MinIO returns a typed error; simplest portable check:
		// treat "not found" as (false, nil); otherwise return the error
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" || errResp.StatusCode == 404 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
