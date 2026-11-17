package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

type SeaweedFSAdapter struct {
	client          *s3.Client
	presignClient   *s3.PresignClient
	bucket          string
	endpoint        string
	presignEndpoint string
}

func NewSeaweedFS(endpoint, publicEndpoint, accessKey, secretKey, bucket string, secure bool) (*SeaweedFSAdapter, error) {
	internalClient, internalURL, err := newS3Client(endpoint, accessKey, secretKey, secure)
	if err != nil {
		return nil, fmt.Errorf("init SeaweedFS client: %w", err)
	}

	public, err := parsePublicEndpoint(publicEndpoint, secure)
	if err != nil {
		return nil, err
	}

	presignClient := s3.NewPresignClient(internalClient)
	presignEndpoint := internalURL
	if public.host != "" {
		publicClient, publicURL, err := newS3Client(public.host, accessKey, secretKey, public.secure)
		if err != nil {
			return nil, fmt.Errorf("init public SeaweedFS client: %w", err)
		}
		presignClient = s3.NewPresignClient(publicClient)
		presignEndpoint = publicURL
	}

	return &SeaweedFSAdapter{
		client:          internalClient,
		presignClient:   presignClient,
		bucket:          bucket,
		endpoint:        internalURL,
		presignEndpoint: presignEndpoint,
	}, nil
}

func newS3Client(endpoint, accessKey, secretKey string, secure bool) (*s3.Client, string, error) {
	clientURL, err := normalizeClientURL(endpoint, secure)
	if err != nil {
		return nil, "", err
	}

	client := s3.New(s3.Options{
		Region:       "us-east-1",
		Credentials:  aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
		BaseEndpoint: aws.String(clientURL),
		UsePathStyle: true,
	})
	return client, clientURL, nil
}

func normalizeClientURL(endpoint string, secure bool) (string, error) {
	value := strings.TrimSpace(endpoint)
	if value == "" {
		return "", fmt.Errorf("SeaweedFS endpoint is required")
	}
	if !strings.Contains(value, "://") {
		scheme := "http"
		if secure {
			scheme = "https"
		}
		value = scheme + "://" + value
	}

	u, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("parse SeaweedFS endpoint: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("SeaweedFS endpoint must include a host")
	}
	if u.Path != "" && u.Path != "/" {
		return "", fmt.Errorf("SeaweedFS endpoint must not include a path")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("SeaweedFS endpoint scheme must be http or https")
	}
	return u.Scheme + "://" + u.Host, nil
}

func (m *SeaweedFSAdapter) PresignPut(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	req, err := m.presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("generate presigned PUT URL: %w", err)
	}
	return req.URL, nil
}

func (m *SeaweedFSAdapter) PresignGet(ctx context.Context, bucket string, key string, expiry time.Duration) (string, error) {
	req, err := m.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("generate presigned GET URL: %w", err)
	}
	return req.URL, nil
}

func (m *SeaweedFSAdapter) Delete(ctx context.Context, bucket, key string) error {
	_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (m *SeaweedFSAdapter) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound") {
			return false, nil
		}
		return false, fmt.Errorf("stat object %q: %w", key, err)
	}
	return true, nil
}

func (m *SeaweedFSAdapter) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("get object %q: %w", key, err)
	}
	return out.Body, nil
}

func (m *SeaweedFSAdapter) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := m.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("put object %q: %w", key, err)
	}
	return nil
}

func (m *SeaweedFSAdapter) Ping(ctx context.Context) error {
	_, err := m.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(m.bucket)})
	if err != nil {
		return fmt.Errorf("check SeaweedFS bucket %q: %w", m.bucket, err)
	}
	return nil
}

type parsedPublicEndpoint struct {
	raw    string
	host   string
	secure bool
}

func parsePublicEndpoint(raw string, defaultSecure bool) (parsedPublicEndpoint, error) {
	if strings.TrimSpace(raw) == "" {
		return parsedPublicEndpoint{}, nil
	}

	endpoint := strings.TrimSpace(raw)
	if !strings.Contains(endpoint, "://") {
		endpoint = "//" + endpoint
	}
	public, err := url.Parse(endpoint)
	if err != nil {
		return parsedPublicEndpoint{}, fmt.Errorf("parse public SeaweedFS endpoint: %w", err)
	}
	if public.Host == "" {
		return parsedPublicEndpoint{}, fmt.Errorf("public SeaweedFS endpoint must include a host")
	}
	if public.Path != "" && public.Path != "/" {
		return parsedPublicEndpoint{}, fmt.Errorf("public SeaweedFS endpoint must not include a path")
	}

	secure := defaultSecure
	switch public.Scheme {
	case "":
	case "http":
		secure = false
	case "https":
		secure = true
	default:
		return parsedPublicEndpoint{}, fmt.Errorf("public SeaweedFS endpoint scheme must be http or https")
	}

	scheme := "http"
	if secure {
		scheme = "https"
	}
	return parsedPublicEndpoint{
		raw:    scheme + "://" + public.Host,
		host:   public.Host,
		secure: secure,
	}, nil
}
