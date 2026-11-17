package services

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeCustomerRepo struct {
	byEmail map[string]models.Customer
}

func newFakeCustomerRepo() *fakeCustomerRepo {
	return &fakeCustomerRepo{byEmail: map[string]models.Customer{}}
}

func (f *fakeCustomerRepo) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeCustomerRepo) Insert(ctx context.Context, customer models.Customer) (string, error) {
	if _, ok := f.byEmail[customer.Email]; ok {
		return "", mongo.ErrNoDocuments
	}
	if customer.ID.IsZero() {
		customer.ID = primitive.NewObjectID()
	}
	f.byEmail[customer.Email] = customer
	return customer.ID.Hex(), nil
}
func (f *fakeCustomerRepo) FindByEmail(ctx context.Context, email string) (models.Customer, error) {
	customer, ok := f.byEmail[email]
	if !ok {
		return models.Customer{}, mongo.ErrNoDocuments
	}
	return customer, nil
}
func (f *fakeCustomerRepo) FindByID(ctx context.Context, id string) (models.Customer, error) {
	for _, customer := range f.byEmail {
		if customer.ID.Hex() == id {
			return customer, nil
		}
	}
	return models.Customer{}, mongo.ErrNoDocuments
}

type fakeCustomerSessionRepo struct {
	sessions map[string]models.CustomerSession
}

func newFakeCustomerSessionRepo() *fakeCustomerSessionRepo {
	return &fakeCustomerSessionRepo{sessions: map[string]models.CustomerSession{}}
}

func (f *fakeCustomerSessionRepo) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeCustomerSessionRepo) Insert(ctx context.Context, session models.CustomerSession) error {
	f.sessions[session.TokenHash] = session
	return nil
}
func (f *fakeCustomerSessionRepo) FindByTokenHash(ctx context.Context, tokenHash string) (models.CustomerSession, error) {
	session, ok := f.sessions[tokenHash]
	if !ok {
		return models.CustomerSession{}, mongo.ErrNoDocuments
	}
	return session, nil
}
func (f *fakeCustomerSessionRepo) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	delete(f.sessions, tokenHash)
	return nil
}

func TestCustomerSignupLoginAndSession(t *testing.T) {
	customers := newFakeCustomerRepo()
	sessions := newFakeCustomerSessionRepo()
	svc := NewCustomerService(config.CustomerConfig{SessionTTL: time.Hour}, customers, sessions)

	customer, token, csrf, _, err := svc.Signup(context.Background(), " Person@Example.com ", "a-long-password", "127.0.0.1")
	if err != nil {
		t.Fatalf("Signup() error = %v", err)
	}
	if customer.Email != "person@example.com" {
		t.Fatalf("Signup() email = %q", customer.Email)
	}
	if token == "" || csrf == "" {
		t.Fatal("Signup() did not return session and csrf tokens")
	}

	authed, session, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authed.ID != customer.ID || !svc.ValidateCSRF(session, csrf) {
		t.Fatal("Authenticate() returned the wrong customer or CSRF validation failed")
	}

	if _, _, _, _, err := svc.Login(context.Background(), "person@example.com", "wrong-password", "127.0.0.1"); !errors.Is(err, ErrCustomerInvalidCredentials) {
		t.Fatalf("Login() bad password error = %v", err)
	}
	if _, _, _, _, err := svc.Login(context.Background(), "person@example.com", "a-long-password", "127.0.0.1"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
}

func TestCustomerEventStartsPendingReview(t *testing.T) {
	repo := &fakeEventRepository{}
	svc := NewEventService(repo)
	plan := DefaultEventPlans()[1]

	res, err := svc.CreateCustomerEvent(context.Background(), "owner-a", eventDto.CreateCustomerEventReq{
		Name:            "Pending Event",
		PlanCode:        plan.Code,
		ContactEmail:    "owner@example.test",
		ContactWhatsApp: "+628123456789",
	}, plan)
	if err != nil {
		t.Fatalf("CreateCustomerEvent() error = %v", err)
	}
	if res.BillingState != models.EventBillingPendingReview || res.Status != models.EventStatusDraft {
		t.Fatalf("CreateCustomerEvent() state = %s/%s, want pending_review/draft", res.BillingState, res.Status)
	}
	if res.OwnerType != models.EventOwnerCustomer || res.OwnerID != "owner-a" {
		t.Fatalf("CreateCustomerEvent() owner = %s/%s", res.OwnerType, res.OwnerID)
	}
	if res.Plan.PhotoQuota != plan.PhotoQuota || res.Plan.BoomerangQuota != plan.BoomerangQuota {
		t.Fatalf("CreateCustomerEvent() plan snapshot = %+v", res.Plan)
	}
	if !res.UploadPolicy.RequireApproval {
		t.Fatal("CreateCustomerEvent() should require showcase approval by default")
	}
}

func TestUpdatePlanToPendingDisablesLiveEvent(t *testing.T) {
	event := models.Event{
		ID:           primitive.NewObjectID(),
		OwnerType:    models.EventOwnerCustomer,
		OwnerID:      "owner-a",
		BillingState: models.EventBillingActive,
		Status:       models.EventStatusLive,
		PlanSnapshot: models.EventPlanSnapshot{Code: "starter", PhotoQuota: 10, BoomerangQuota: 5},
	}
	repo := &fakeEventRepository{event: event}
	svc := NewEventService(repo)

	res, err := svc.UpdatePlan(context.Background(), event.ID.Hex(), eventDto.UpdatePlanReq{BillingState: "pending_review"}, nil)
	if err != nil {
		t.Fatalf("UpdatePlan() error = %v", err)
	}
	if res.Status != models.EventStatusDraft {
		t.Fatalf("UpdatePlan() status = %q, want draft", res.Status)
	}
}

func TestCustomerCannotReadAnotherCustomerEvent(t *testing.T) {
	event := models.Event{
		ID:        primitive.NewObjectID(),
		OwnerType: models.EventOwnerCustomer,
		OwnerID:   "owner-a",
		Name:      "Owned Event",
		Slug:      "owned-event",
	}
	repo := &fakeEventRepository{event: event}
	svc := NewEventService(repo)

	if _, err := svc.GetCustomerEvent(context.Background(), event.ID.Hex(), "owner-b"); !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("GetCustomerEvent() error = %v, want ErrEventNotFound", err)
	}
	if _, err := svc.GetCustomerEvent(context.Background(), event.ID.Hex(), "owner-a"); err != nil {
		t.Fatalf("GetCustomerEvent() owner error = %v", err)
	}
}

func TestQuotaConsumptionAndRelease(t *testing.T) {
	event := models.Event{
		ID:           primitive.NewObjectID(),
		PlanSnapshot: models.EventPlanSnapshot{Code: "trial", PhotoQuota: 1, BoomerangQuota: 0},
	}
	repo := &fakeEventRepository{event: event}

	ok, err := repo.ConsumeQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto, false)
	if err != nil || !ok {
		t.Fatalf("ConsumeQuota() = %v, %v", ok, err)
	}
	ok, err = repo.ConsumeQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto, false)
	if err != nil || ok {
		t.Fatalf("ConsumeQuota() over limit = %v, %v", ok, err)
	}
	if err := repo.ReleaseQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto); err != nil {
		t.Fatalf("ReleaseQuota() error = %v", err)
	}
	ok, err = repo.ConsumeQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto, false)
	if err != nil || !ok {
		t.Fatalf("ConsumeQuota() after release = %v, %v", ok, err)
	}
	ok, err = repo.ConsumeQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto, false)
	if err != nil || ok {
		t.Fatalf("ConsumeQuota() second over limit = %v, %v", ok, err)
	}
	ok, err = repo.ConsumeQuota(context.Background(), event.ID.Hex(), models.MediaKindPhoto, true)
	if err != nil || !ok {
		t.Fatalf("ConsumeQuota() with over-quota override = %v, %v", ok, err)
	}
	if repo.event.UsedPhotos != 2 {
		t.Fatalf("UsedPhotos = %d, want 2", repo.event.UsedPhotos)
	}
}

func TestDefaultEventPlans(t *testing.T) {
	plans := DefaultEventPlans()
	if len(plans) != 4 {
		t.Fatalf("DefaultEventPlans() = %d plans, want 4", len(plans))
	}
	seen := map[string]bool{}
	for _, plan := range plans {
		if plan.Code == "" || plan.Currency != "IDR" || plan.PhotoQuota < 0 || plan.BoomerangQuota < 0 {
			t.Fatalf("invalid plan: %+v", plan)
		}
		if seen[plan.Code] {
			t.Fatalf("duplicate plan code %q", plan.Code)
		}
		seen[strings.ToLower(plan.Code)] = true
	}
}
