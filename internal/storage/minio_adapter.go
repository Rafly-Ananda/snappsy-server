package storage

import (
	"context"
	"os"
	"strings"
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
	appEnv := os.Getenv("APP_ENV") // production, development, testing
	u, err := m.Client.PresignedPutObject(ctx, bucket, key, expiry)
	if err != nil {
		return "", err
	}

	publicURL := os.Getenv("MINIO_PUBLIC_URL") // https://files.rafly.com
    internalEndpoint := os.Getenv("MINIO_ENDPOINT") // http://minio:9000

	urlStr := u.String()
	// Replace http://minio:9000 with https://files.raflysoemantri.cloud
	urlStr = strings.Replace(urlStr, "http://"+internalEndpoint, publicURL, 1)
	// Also handle https case
	urlStr = strings.Replace(urlStr, "https://"+internalEndpoint, publicURL, 1)

	if (appEnv == "production") {
		return urlStr, nil
	}

	return u.String(), nil
}

func (m *MinioAdapter) PresignGet(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	u, err := m.Client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}

    publicURL := os.Getenv("MINIO_PUBLIC_URL") // https://files.rafly.com
    internalEndpoint := os.Getenv("MINIO_ENDPOINT") // http://minio:9000

	urlStr := u.String()
    urlStr = strings.Replace(urlStr, internalEndpoint, publicURL, 1)

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
