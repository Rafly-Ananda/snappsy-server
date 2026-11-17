package services

import (
	"bytes"
	"context"
	"errors"
	"io"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakeEventRepository struct {
	event models.Event
}

func (f *fakeEventRepository) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeEventRepository) Insert(ctx context.Context, event models.Event) (string, error) {
	if event.ID.IsZero() {
		event.ID = primitive.NewObjectID()
	}
	f.event = event
	return event.ID.Hex(), nil
}
func (f *fakeEventRepository) Update(ctx context.Context, event models.Event) error {
	f.event = event
	return nil
}
func (f *fakeEventRepository) FindByID(ctx context.Context, id string) (models.Event, error) {
	if f.event.ID.IsZero() || f.event.ID.Hex() != id {
		return models.Event{}, errors.New("event not found")
	}
	return f.event, nil
}
func (f *fakeEventRepository) FindBySlug(ctx context.Context, slug string) (models.Event, error) {
	if f.event.Slug != slug {
		return models.Event{}, errors.New("event not found")
	}
	return f.event, nil
}
func (f *fakeEventRepository) FindByDisplayTokenHash(ctx context.Context, hash string) (models.Event, error) {
	if f.event.DisplayTokenHash != hash {
		return models.Event{}, errors.New("event not found")
	}
	return f.event, nil
}
func (f *fakeEventRepository) List(ctx context.Context) ([]models.Event, error) {
	if f.event.ID.IsZero() {
		return nil, nil
	}
	return []models.Event{f.event}, nil
}
func (f *fakeEventRepository) ListByOwner(ctx context.Context, ownerType models.EventOwnerType, ownerID string) ([]models.Event, error) {
	if f.event.ID.IsZero() || f.event.OwnerType != ownerType || f.event.OwnerID != ownerID {
		return nil, nil
	}
	return []models.Event{f.event}, nil
}
func (f *fakeEventRepository) ConsumeQuota(ctx context.Context, id string, kind models.MediaKind, allowOverQuota bool) (bool, error) {
	if f.event.ID.Hex() != id {
		return false, nil
	}
	if f.event.PlanSnapshot.Code == "" {
		return true, nil
	}
	quota := f.event.PlanSnapshot.PhotoQuota
	used := f.event.UsedPhotos
	if kind == models.MediaKindBoomerang {
		quota = f.event.PlanSnapshot.BoomerangQuota
		used = f.event.UsedBoomerangs
	}
	if !allowOverQuota && quota != -1 && used >= quota {
		return false, nil
	}
	if kind == models.MediaKindBoomerang {
		f.event.UsedBoomerangs++
	} else {
		f.event.UsedPhotos++
	}
	return true, nil
}
func (f *fakeEventRepository) ClearTemplateReference(ctx context.Context, templateID string) error {
	return nil
}

func (f *fakeEventRepository) UpdateTemplateVersion(ctx context.Context, templateID string, version int) error {
	return nil
}

func (f *fakeEventRepository) ReleaseQuota(ctx context.Context, id string, kind models.MediaKind) error {
	if kind == models.MediaKindBoomerang && f.event.UsedBoomerangs > 0 {
		f.event.UsedBoomerangs--
	} else if kind != models.MediaKindBoomerang && f.event.UsedPhotos > 0 {
		f.event.UsedPhotos--
	}
	return nil
}

func newLiveEvent() *fakeEventRepository {
	return &fakeEventRepository{event: models.Event{
		ID:           primitive.NewObjectID(),
		Name:         "Test Event",
		Slug:         "test-event",
		Status:       models.EventStatusLive,
		CaptureModes: models.EventCaptureModes{Photo: true, Strip: true, Boomerang: true},
		UploadPolicy: models.EventUploadPolicy{
			AcceptUploads: true,
		},
	}}
}

type fakeObjectStorage struct {
	exists  bool
	deleted []string
	keys    []string
}

func (f *fakeObjectStorage) PresignPut(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	f.keys = append(f.keys, key)
	return "https://example.test/" + key, nil
}
func (f *fakeObjectStorage) PresignGet(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	return "https://example.test/" + key, nil
}
func (f *fakeObjectStorage) Delete(ctx context.Context, bucket, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}
func (f *fakeObjectStorage) Exists(ctx context.Context, bucket, key string) (bool, error) {
	return f.exists, nil
}
func (f *fakeObjectStorage) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader([]byte("media"))), nil
}
func (f *fakeObjectStorage) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	return nil
}
