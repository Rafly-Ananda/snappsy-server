package services

import (
	"context"
	"errors"
	"testing"
	"time"

	photoDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type fakePhotoMediaRepo struct {
	items    []models.MediaItem
	expired  []models.MediaItem
	byID     map[string]models.MediaItem
	byIntent map[string]models.MediaItem
}

func (f *fakePhotoMediaRepo) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakePhotoMediaRepo) Insert(ctx context.Context, item models.MediaItem) (string, error) {
	if item.ID.IsZero() {
		item.ID = primitive.NewObjectID()
	}
	if f.byIntent == nil {
		f.byIntent = make(map[string]models.MediaItem)
	}
	if f.byID == nil {
		f.byID = make(map[string]models.MediaItem)
	}
	f.items = append(f.items, item)
	f.byID[item.ID.Hex()] = item
	f.byIntent[item.UploadIntentID] = item
	return item.ID.Hex(), nil
}
func (f *fakePhotoMediaRepo) Update(ctx context.Context, item models.MediaItem) error {
	f.byID[item.ID.Hex()] = item
	return nil
}
func (f *fakePhotoMediaRepo) FindByID(ctx context.Context, id string) (models.MediaItem, error) {
	item, ok := f.byID[id]
	if !ok {
		return models.MediaItem{}, errors.New("not found")
	}
	return item, nil
}
func (f *fakePhotoMediaRepo) FindByUploadIntentID(ctx context.Context, intentID string) (models.MediaItem, error) {
	item, ok := f.byIntent[intentID]
	if !ok {
		return models.MediaItem{}, errors.New("not found")
	}
	return item, nil
}
func (f *fakePhotoMediaRepo) FindExpired(ctx context.Context, now time.Time, limit int) ([]models.MediaItem, error) {
	return f.expired, nil
}
func (f *fakePhotoMediaRepo) FindReadyByEvent(ctx context.Context, eventId string) ([]models.MediaItem, error) {
	return f.items, nil
}
func (f *fakePhotoMediaRepo) Delete(ctx context.Context, id string) error {
	delete(f.byID, id)
	return nil
}
func (f *fakePhotoMediaRepo) FindAllByEvents(ctx context.Context, eventId string, moderationState string, cursor string, limit int) ([]models.MediaItem, string, error) {
	out := make([]models.MediaItem, 0)
	for _, item := range f.items {
		if item.EventID == eventId {
			out = append(out, item)
		}
	}
	return out, "", nil
}

type fakeIntentRepo struct {
	intents map[string]models.UploadIntent
}

func (f *fakeIntentRepo) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeIntentRepo) Insert(ctx context.Context, intent models.UploadIntent) (string, error) {
	if f.intents == nil {
		f.intents = make(map[string]models.UploadIntent)
	}
	id := intent.ID.Hex()
	f.intents[id] = intent
	return id, nil
}
func (f *fakeIntentRepo) FindByID(ctx context.Context, id string) (models.UploadIntent, error) {
	intent, ok := f.intents[id]
	if !ok {
		return models.UploadIntent{}, errors.New("not found")
	}
	return intent, nil
}
func (f *fakeIntentRepo) Complete(ctx context.Context, id string, mediaID string) error {
	intent, ok := f.intents[id]
	if !ok {
		return errors.New("not found")
	}
	intent.MediaID = mediaID
	f.intents[id] = intent
	return nil
}

func newPhotoService(t *testing.T) (*PhotoService, *fakePhotoMediaRepo, *fakeIntentRepo, *fakeEventRepository) {
	t.Helper()
	media := &fakePhotoMediaRepo{}
	intents := &fakeIntentRepo{}
	events := newLiveEvent()
	svc := NewPhotoService(media, intents, events, &fakeTemplateRepository{}, &fakeAssetRepository{}, &fakeObjectStorage{exists: true}, "images", time.Minute, nil)
	return svc, media, intents, events
}

func TestCreatePhotoIntentUsesScopedRandomKeys(t *testing.T) {
	svc, _, intents, events := newPhotoService(t)
	res, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{
		EventID:  events.event.ID.Hex(),
		MimeType: "image/jpeg",
	})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	if res.IntentID == "" || res.StatusToken == "" || res.BaseKey == res.CompositeKey {
		t.Fatalf("CreateIntent() response = %+v", res)
	}
	if len(intents.intents) != 1 {
		t.Fatalf("CreateIntent() stored %d intents", len(intents.intents))
	}
}

func TestCommitPhotoRequiresConsentAndUploadedObjects(t *testing.T) {
	svc, _, _, events := newPhotoService(t)
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	_, err = svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID:    created.IntentID,
		StatusToken: created.StatusToken,
		Username:    "Guest",
		Orientation: "portrait",
		Consent:     false,
	})
	if !errors.Is(err, ErrPhotoConsentRequired) {
		t.Fatalf("Commit() consent error = %v", err)
	}

	svc.obj = &fakeObjectStorage{exists: false}
	_, err = svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID:    created.IntentID,
		StatusToken: created.StatusToken,
		Username:    "Guest",
		Orientation: "portrait",
		Consent:     true,
	})
	if !errors.Is(err, ErrUploadedObjectNotFound) {
		t.Fatalf("Commit() missing object error = %v", err)
	}
}

func TestCommitPhotoIsIdempotentAndRespectsApprovalPolicy(t *testing.T) {
	svc, media, _, events := newPhotoService(t)
	events.event.UploadPolicy.RequireApproval = true
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/png"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	req := photoDto.CommitReq{
		IntentID:    created.IntentID,
		StatusToken: created.StatusToken,
		Username:    "Guest",
		Captions:    "Hello",
		Orientation: "landscape",
		Consent:     true,
	}
	first, err := svc.Commit(context.Background(), req)
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	second, err := svc.Commit(context.Background(), req)
	if err != nil {
		t.Fatalf("Commit() retry error = %v", err)
	}
	if first.ID != second.ID || len(media.items) != 1 {
		t.Fatalf("Commit() was not idempotent: first=%s second=%s items=%d", first.ID, second.ID, len(media.items))
	}
	if first.ModerationState != models.MediaModerationPending {
		t.Fatalf("Commit() moderation = %q", first.ModerationState)
	}

	status, err := svc.Status(context.Background(), first.ID, created.StatusToken)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.URL == "" || status.ProcessingState != models.MediaProcessingReady {
		t.Fatalf("Status() = %+v", status)
	}
	if status.DownloadURL != "" {
		t.Fatal("Status() exposed download when policy disallows it")
	}
}

func TestCommitPhotoRejectsDisabledCaptureMode(t *testing.T) {
	svc, media, _, events := newPhotoService(t)
	events.event.CaptureModes.Photo = false
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	_, err = svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Orientation: "portrait", Consent: true,
	})
	if !errors.Is(err, ErrCaptureModeDisabled) {
		t.Fatalf("Commit() disabled mode error = %v", err)
	}
	if len(media.items) != 0 {
		t.Fatalf("Commit() stored %d disabled media items", len(media.items))
	}
}

func TestCommitPhotoStripConsumesOnePhotoQuota(t *testing.T) {
	svc, media, _, events := newPhotoService(t)
	events.event.PlanSnapshot = models.EventPlanSnapshot{Code: "starter", PhotoQuota: 1}
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	_, err = svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Orientation: "portrait", Kind: string(models.MediaKindPhotoStrip), Consent: true,
	})
	if err != nil {
		t.Fatalf("Commit() strip error = %v", err)
	}
	if len(media.items) != 1 || media.items[0].Kind != models.MediaKindPhotoStrip {
		t.Fatalf("Commit() media = %+v", media.items)
	}
	if events.event.UsedPhotos != 1 || events.event.UsedBoomerangs != 0 {
		t.Fatalf("Commit() usage = photos:%d boomerangs:%d", events.event.UsedPhotos, events.event.UsedBoomerangs)
	}

	second, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() second error = %v", err)
	}
	_, err = svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID: second.IntentID, StatusToken: second.StatusToken, Username: "Guest", Orientation: "portrait", Consent: true,
	})
	if !errors.Is(err, ErrEventQuotaExceeded) {
		t.Fatalf("Commit() quota error = %v", err)
	}
}

func TestCustomerCannotListOrModerateAnotherCustomersMedia(t *testing.T) {
	svc, media, _, events := newPhotoService(t)
	events.event.OwnerType = models.EventOwnerCustomer
	events.event.OwnerID = "owner-a"
	itemID, err := media.Insert(context.Background(), models.MediaItem{
		ID: primitive.NewObjectID(), EventID: events.event.ID.Hex(), Kind: models.MediaKindPhoto,
		ModerationState: models.MediaModerationPending, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	if _, err := svc.ListForCustomerEvent(context.Background(), events.event.ID.Hex(), "owner-b", "", 24, ""); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("ListForCustomerEvent() cross-owner error = %v", err)
	}
	if _, err := svc.ListForCustomerEvent(context.Background(), events.event.ID.Hex(), "owner-a", "", 24, ""); err != nil {
		t.Fatalf("ListForCustomerEvent() owner error = %v", err)
	}
	if err := svc.SetModerationStateForOwner(context.Background(), itemID, "owner-b", string(models.MediaModerationApproved)); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("SetModerationStateForOwner() cross-owner error = %v", err)
	}
	if err := svc.SetModerationStateForOwner(context.Background(), itemID, "owner-a", string(models.MediaModerationApproved)); err != nil {
		t.Fatalf("SetModerationStateForOwner() owner error = %v", err)
	}
}

func TestPhotoDownloadRequiresPolicy(t *testing.T) {
	svc, _, _, events := newPhotoService(t)
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	saved, err := svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Orientation: "portrait", Consent: true,
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := svc.Download(context.Background(), saved.ID, created.StatusToken); !errors.Is(err, ErrPhotoDownloadNotAllowed) {
		t.Fatalf("Download() error = %v", err)
	}

	events.event.UploadPolicy.AllowDownload = true
	if _, err := svc.Download(context.Background(), saved.ID, created.StatusToken); err != nil {
		t.Fatalf("Download() allowed error = %v", err)
	}
}

func TestPhotoStatusRejectsWrongToken(t *testing.T) {
	svc, _, _, events := newPhotoService(t)
	created, err := svc.CreateIntent(context.Background(), photoDto.CreateIntentReq{EventID: events.event.ID.Hex(), MimeType: "image/jpeg"})
	if err != nil {
		t.Fatalf("CreateIntent() error = %v", err)
	}
	saved, err := svc.Commit(context.Background(), photoDto.CommitReq{
		IntentID: created.IntentID, StatusToken: created.StatusToken, Username: "Guest", Orientation: "portrait", Consent: true,
	})
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if _, err := svc.Status(context.Background(), saved.ID, "wrong-token"); !errors.Is(err, ErrPhotoStatusTokenInvalid) {
		t.Fatalf("Status() error = %v", err)
	}
}
