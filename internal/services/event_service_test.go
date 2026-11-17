package services

import (
	"context"
	"testing"

	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCreateEventReturnsSlugAndOneTimeDisplayToken(t *testing.T) {
	repo := &fakeEventRepository{}
	svc := NewEventService(repo)

	res, err := svc.CreateEvent(context.Background(), eventDto.CreateEventReq{
		Name:        "Rafly & Intan Wedding",
		Description: "Existing wedding event",
	})
	if err != nil {
		t.Fatalf("CreateEvent() error = %v", err)
	}
	if res.ID == "" || res.Slug == "" || res.DisplayToken == "" {
		t.Fatalf("CreateEvent() response = %+v", res)
	}
	if res.Status != models.EventStatusDraft {
		t.Fatalf("CreateEvent() status = %q", res.Status)
	}
}

func TestPublishAndArchiveEvent(t *testing.T) {
	repo := &fakeEventRepository{event: models.Event{
		ID:     primitive.NewObjectID(),
		Status: models.EventStatusDraft,
		UploadPolicy: models.EventUploadPolicy{
			AcceptUploads: true,
		},
	}}
	svc := NewEventService(repo)

	published, err := svc.PublishEvent(context.Background(), repo.event.ID.Hex())
	if err != nil {
		t.Fatalf("PublishEvent() error = %v", err)
	}
	if published.Status != models.EventStatusLive {
		t.Fatalf("PublishEvent() status = %q", published.Status)
	}

	archived, err := svc.ArchiveEvent(context.Background(), repo.event.ID.Hex())
	if err != nil {
		t.Fatalf("ArchiveEvent() error = %v", err)
	}
	if archived.Status != models.EventStatusArchived {
		t.Fatalf("ArchiveEvent() status = %q", archived.Status)
	}
}

func TestRotateDisplayTokenReturnsNewRawToken(t *testing.T) {
	repo := &fakeEventRepository{event: models.Event{
		ID:     primitive.NewObjectID(),
		Status: models.EventStatusLive,
		UploadPolicy: models.EventUploadPolicy{
			AcceptUploads: true,
		},
	}}
	svc := NewEventService(repo)

	res, err := svc.RotateDisplayToken(context.Background(), repo.event.ID.Hex())
	if err != nil {
		t.Fatalf("RotateDisplayToken() error = %v", err)
	}
	if res.DisplayToken == "" {
		t.Fatal("RotateDisplayToken() returned empty token")
	}
}
