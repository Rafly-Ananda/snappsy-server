package services

import (
	"context"
	"errors"
	"testing"
	"time"

	boomerangDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/boomerangs"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type fakeMediaJobRepo struct {
	jobs   []models.MediaJob
	active int64
}

func (f *fakeMediaJobRepo) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeMediaJobRepo) Insert(ctx context.Context, job models.MediaJob) error {
	f.jobs = append(f.jobs, job)
	return nil
}
func (f *fakeMediaJobRepo) ClaimNext(ctx context.Context) (models.MediaJob, error) {
	return models.MediaJob{}, errors.New("no jobs")
}
func (f *fakeMediaJobRepo) CountActiveByEvent(ctx context.Context, eventID string) (int64, error) {
	return f.active, nil
}
func (f *fakeMediaJobRepo) SummaryByEvent(ctx context.Context, eventID string) (int64, int64, error) {
	return f.active, 0, nil
}
func (f *fakeMediaJobRepo) Complete(ctx context.Context, id string) error             { return nil }
func (f *fakeMediaJobRepo) DeleteByMediaID(ctx context.Context, mediaID string) error { return nil }
func (f *fakeMediaJobRepo) Fail(ctx context.Context, id string, code string) error {
	return nil
}

func newBoomerangService(t *testing.T, queueLimit int) (*BoomerangService, *fakePhotoMediaRepo, *fakeIntentRepo, *fakeMediaJobRepo, *fakeEventRepository) {
	t.Helper()
	media := &fakePhotoMediaRepo{}
	intents := &fakeIntentRepo{}
	jobs := &fakeMediaJobRepo{}
	events := newLiveEvent()
	svc := NewBoomerangService(media, intents, jobs, events, &fakeObjectStorage{exists: true}, "images", time.Minute, 3, 50*1024*1024, queueLimit)
	return svc, media, intents, jobs, events
}

func TestCreateBoomerangIntentAcceptsVideo(t *testing.T) {
	svc, _, intents, _, events := newBoomerangService(t, 10)
	res, err := svc.CreateIntent(context.Background(), boomerangDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "video/mp4"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	if res.IntentID == "" || res.StatusToken == "" || res.SourceKey == "" || res.MaxSeconds != 3 {
		t.Fatalf("CreateIntent() = %+v", res)
	}
	if len(intents.intents) != 1 {
		t.Fatalf("CreateIntent() stored %d intents", len(intents.intents))
	}
}

func TestCreateBoomerangIntentRejectsInvalidType(t *testing.T) {
	svc, _, _, _, events := newBoomerangService(t, 10)
	_, err := svc.CreateIntent(context.Background(), boomerangDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if !errors.Is(err, ErrBoomerangInvalidSource) {
		t.Fatalf("CreateIntent() error = %v", err)
	}
}

func TestCommitBoomerangQueuesJob(t *testing.T) {
	svc, media, _, jobs, events := newBoomerangService(t, 10)
	created, err := svc.CreateIntent(context.Background(), boomerangDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "video/webm"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	res, err := svc.Commit(context.Background(), boomerangDto.CommitReq{
		IntentID:       created.IntentID,
		StatusToken:    created.StatusToken,
		Username:       "Guest",
		Consent:        true,
		StartSeconds:   0,
		SegmentSeconds: 3,
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if res.ProcessingState != models.MediaProcessingProcessing || len(media.items) != 1 || len(jobs.jobs) != 1 {
		t.Fatalf("Commit() res=%+v media=%d jobs=%d", res, len(media.items), len(jobs.jobs))
	}
	if media.items[0].Kind != models.MediaKindBoomerang {
		t.Fatalf("Commit() kind = %q", media.items[0].Kind)
	}
}

func TestCommitBoomerangRejectsInvalidSegmentAndQueue(t *testing.T) {
	svc, _, _, jobs, events := newBoomerangService(t, 1)
	created, err := svc.CreateIntent(context.Background(), boomerangDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "video/mp4"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	_, err = svc.Commit(context.Background(), boomerangDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Consent: true, SegmentSeconds: 4,
	})
	if !errors.Is(err, ErrBoomerangInvalidSegment) {
		t.Fatalf("Commit() segment error = %v", err)
	}
	jobs.active = 1
	_, err = svc.Commit(context.Background(), boomerangDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Consent: true, SegmentSeconds: 3,
	})
	if !errors.Is(err, ErrBoomerangQueueFull) {
		t.Fatalf("Commit() queue error = %v", err)
	}
}

func TestBoomerangStatusExposesPlaybackWhenReady(t *testing.T) {
	svc, media, _, _, events := newBoomerangService(t, 10)
	events.event.UploadPolicy.AllowDownload = true
	created, err := svc.CreateIntent(context.Background(), boomerangDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "video/mp4"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	res, err := svc.Commit(context.Background(), boomerangDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Consent: true, SegmentSeconds: 3,
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := svc.Download(context.Background(), res.ID, created.StatusToken); !errors.Is(err, ErrPhotoNotReady) {
		t.Fatalf("Download() before ready = %v", err)
	}
	item := media.items[0]
	item.ProcessingState = models.MediaProcessingReady
	item.OutputKey = "events/e/loop.mp4"
	item.PosterKey = "events/e/poster.jpg"
	media.items[0] = item
	media.byID[res.ID] = item
	status, err := svc.Status(context.Background(), res.ID, created.StatusToken)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.PlaybackURL == "" || status.DownloadURL == "" || status.URL == "" {
		t.Fatalf("Status() = %+v", status)
	}
}
