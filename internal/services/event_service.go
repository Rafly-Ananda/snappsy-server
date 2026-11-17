package services

import (
	"context"
	"errors"
	"strings"
	"time"

	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrEventNotFound            = errors.New("event not found")
	ErrEventNotLive             = errors.New("event is not live")
	ErrEventNotAcceptingUploads = errors.New("event is not accepting uploads")
	ErrEventNotApproved         = errors.New("event plan is not active")
	ErrEventQuotaExceeded       = errors.New("event media quota exceeded")
	ErrCaptureModeDisabled      = errors.New("this capture mode is disabled for the event")
)

const unlimitedQuota = -1

type EventService struct {
	repo   repositories.EventRepository
	design DesignValidator
}

func NewEventService(repo repositories.EventRepository, design ...DesignValidator) *EventService {
	service := &EventService{repo: repo}
	if len(design) > 0 {
		service.design = design[0]
	}
	return service
}

func (s *EventService) CreateEvent(ctx context.Context, req eventDto.CreateEventReq) (eventDto.CreateEventRes, error) {
	events, err := s.repo.List(ctx)
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	used := make(map[string]bool, len(events))
	for _, item := range events {
		used[item.Slug] = true
	}

	now := time.Now()
	id := primitive.NewObjectID()
	event := models.Event{
		ID:           id,
		OwnerType:    models.EventOwnerPlatform,
		BillingState: models.EventBillingActive,
		PlanCode:     "platform",
		PlanSnapshot: models.EventPlanSnapshot{
			Code: "platform", Name: "Platform", Currency: "IDR",
			PhotoQuota: unlimitedQuota, BoomerangQuota: unlimitedQuota, RetentionDays: 30,
		},
		Name:        req.Name,
		Description: req.Description,
		Slug:        identity.UniqueSlug(req.Name, eventIDSuffix(id), used),
		Status:      models.EventStatusDraft,
		UploadPolicy: models.EventUploadPolicy{
			AcceptUploads: true,
		},
		CaptureModes:  models.EventCaptureModes{Photo: true, Strip: true, Boomerang: true},
		Countdowns:    models.EventCountdowns{Photo: 3, Strip: 3, Boomerang: 3},
		RetentionDays: 30,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	// The raw display token is intentionally returned only once at creation/rotation time.
	rawToken, tokenHash, err := identity.NewDisplayToken()
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	event.DisplayTokenHash = tokenHash

	insertedID, err := s.repo.Insert(ctx, event)
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	if oid, err := primitive.ObjectIDFromHex(insertedID); err == nil {
		event.ID = oid
	}

	res := eventDto.CreateEventRes{EventRes: eventDto.NewEventRes(event)}
	res.DisplayToken = rawToken
	return res, nil
}

func (s *EventService) CreateCustomerEvent(ctx context.Context, ownerID string, req eventDto.CreateCustomerEventReq, plan models.EventPlan) (eventDto.CreateEventRes, error) {
	events, err := s.repo.List(ctx)
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	used := make(map[string]bool, len(events))
	for _, item := range events {
		used[item.Slug] = true
	}

	now := time.Now()
	id := primitive.NewObjectID()
	snapshot := plan.Snapshot()
	event := models.Event{
		ID:              id,
		OwnerType:       models.EventOwnerCustomer,
		OwnerID:         ownerID,
		BillingState:    models.EventBillingPendingReview,
		PlanCode:        plan.Code,
		PlanSnapshot:    snapshot,
		ContactEmail:    strings.TrimSpace(req.ContactEmail),
		ContactWhatsApp: strings.TrimSpace(req.ContactWhatsApp),
		Name:            req.Name,
		Description:     req.Description,
		Slug:            identity.UniqueSlug(req.Name, eventIDSuffix(id), used),
		Status:          models.EventStatusDraft,
		UploadPolicy: models.EventUploadPolicy{
			AcceptUploads:   true,
			RequireApproval: true,
		},
		CaptureModes:  models.EventCaptureModes{Photo: true, Strip: true, Boomerang: true},
		Countdowns:    models.EventCountdowns{Photo: 3, Strip: 3, Boomerang: 3},
		RetentionDays: snapshot.RetentionDays,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	rawToken, tokenHash, err := identity.NewDisplayToken()
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	event.DisplayTokenHash = tokenHash

	insertedID, err := s.repo.Insert(ctx, event)
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}
	if oid, err := primitive.ObjectIDFromHex(insertedID); err == nil {
		event.ID = oid
	}
	res := eventDto.CreateEventRes{EventRes: eventDto.NewEventRes(event)}
	res.DisplayToken = rawToken
	return res, nil
}

func (s *EventService) UpdateEvent(ctx context.Context, id string, req eventDto.UpdateEventReq) (eventDto.EventRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	if req.Name != nil {
		event.Name = *req.Name
	}
	if req.Description != nil {
		event.Description = *req.Description
	}
	if req.ContactEmail != nil {
		event.ContactEmail = strings.TrimSpace(*req.ContactEmail)
	}
	if req.ContactWhatsApp != nil {
		event.ContactWhatsApp = strings.TrimSpace(*req.ContactWhatsApp)
	}
	if req.CaptureModes != nil {
		event.CaptureModes = *req.CaptureModes
	}
	if req.StartsAt != nil {
		event.StartsAt = req.StartsAt
	}
	if req.EndsAt != nil {
		event.EndsAt = req.EndsAt
	}
	if req.Branding != nil {
		event.Branding = *req.Branding
	}
	if req.Countdowns != nil {
		event.Countdowns = req.Countdowns.Normalized()
	}
	if req.UploadPolicy != nil {
		event.UploadPolicy = *req.UploadPolicy
	}
	if req.RetentionDays != nil {
		event.RetentionDays = *req.RetentionDays
	}
	if req.DesignVersion != nil {
		event.DesignVersion = *req.DesignVersion
	}
	if req.TemplateID != nil {
		templateID := strings.TrimSpace(*req.TemplateID)
		if templateID == "" {
			event.TemplateID = ""
			event.TemplateVersion = 0
		} else {
			if s.design == nil {
				return eventDto.EventRes{}, ErrTemplateAssignment
			}
			version := 0
			if req.TemplateVersion != nil {
				version = *req.TemplateVersion
			}
			validatedVersion, err := s.design(ctx, templateID, version)
			if err != nil {
				return eventDto.EventRes{}, err
			}
			event.TemplateID = templateID
			event.TemplateVersion = validatedVersion
		}
	}
	if req.StripTemplateID != nil {
		templateID := strings.TrimSpace(*req.StripTemplateID)
		if templateID == "" {
			event.StripTemplateID = ""
			event.StripTemplateVersion = 0
		} else if s.design != nil {
			version := 0
			if req.StripTemplateVersion != nil {
				version = *req.StripTemplateVersion
			}
			validatedVersion, err := s.design(ctx, templateID, version)
			if err != nil {
				return eventDto.EventRes{}, err
			}
			event.StripTemplateID = templateID
			event.StripTemplateVersion = validatedVersion
		}
	}
	if req.BoomerangTemplateID != nil {
		templateID := strings.TrimSpace(*req.BoomerangTemplateID)
		if templateID == "" {
			event.BoomerangTemplateID = ""
			event.BoomerangTemplateVersion = 0
		} else if s.design != nil {
			version := 0
			if req.BoomerangTemplateVersion != nil {
				version = *req.BoomerangTemplateVersion
			}
			validatedVersion, err := s.design(ctx, templateID, version)
			if err != nil {
				return eventDto.EventRes{}, err
			}
			event.BoomerangTemplateID = templateID
			event.BoomerangTemplateVersion = validatedVersion
		}
	}
	if req.StickerAssetIDs != nil {
		event.StickerAssetIDs = *req.StickerAssetIDs
		if req.HiddenStickerIDs == nil {
			event.HiddenStickerIDs = nil
		}
	}
	if req.HiddenStickerIDs != nil {
		event.HiddenStickerIDs = *req.HiddenStickerIDs
	}
	if req.AllowedTemplateIDs != nil {
		event.AllowedTemplateIDs = *req.AllowedTemplateIDs
	}
	event.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, event); err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) GetEvent(ctx context.Context, id string) (eventDto.EventRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) ListEvents(ctx context.Context) ([]eventDto.EventRes, error) {
	events, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]eventDto.EventRes, 0, len(events))
	for _, event := range events {
		out = append(out, eventDto.NewEventRes(event))
	}
	return out, nil
}

func (s *EventService) PublishEvent(ctx context.Context, id string) (eventDto.EventRes, error) {
	return s.setStatus(ctx, id, models.EventStatusLive)
}

func (s *EventService) ArchiveEvent(ctx context.Context, id string) (eventDto.EventRes, error) {
	return s.setStatus(ctx, id, models.EventStatusArchived)
}

func (s *EventService) RotateDisplayToken(ctx context.Context, id string) (eventDto.RotateDisplayTokenRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.RotateDisplayTokenRes{}, err
	}
	raw, hash, err := identity.NewDisplayToken()
	if err != nil {
		return eventDto.RotateDisplayTokenRes{}, err
	}
	event.DisplayTokenHash = hash
	event.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, event); err != nil {
		return eventDto.RotateDisplayTokenRes{}, err
	}
	return eventDto.RotateDisplayTokenRes{DisplayToken: raw}, nil
}

func (s *EventService) ListCustomerEvents(ctx context.Context, ownerID string) ([]eventDto.EventRes, error) {
	events, err := s.repo.ListByOwner(ctx, models.EventOwnerCustomer, ownerID)
	if err != nil {
		return nil, err
	}
	out := make([]eventDto.EventRes, 0, len(events))
	for _, event := range events {
		out = append(out, eventDto.NewEventRes(event))
	}
	return out, nil
}

func (s *EventService) GetCustomerEvent(ctx context.Context, id, ownerID string) (eventDto.EventRes, error) {
	event, err := s.getCustomerEvent(ctx, id, ownerID)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) ClearTemplateReference(ctx context.Context, templateID string) error {
	return s.repo.ClearTemplateReference(ctx, templateID)
}

func (s *EventService) UpdateTemplateVersion(ctx context.Context, templateID string, version int) error {
	return s.repo.UpdateTemplateVersion(ctx, templateID, version)
}

func (s *EventService) ResetTemplate(ctx context.Context, id, kind string) (eventDto.EventRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	switch kind {
	case "photo":
		event.TemplateID = ""
		event.TemplateVersion = 0
	case "strip":
		event.StripTemplateID = ""
		event.StripTemplateVersion = 0
	case "video_frame":
		event.BoomerangTemplateID = ""
		event.BoomerangTemplateVersion = 0
	default:
		return eventDto.EventRes{}, ErrTemplateAssignment
	}
	event.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, event); err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) ResetCustomerTemplate(ctx context.Context, id, ownerID, kind string) (eventDto.EventRes, error) {
	if _, err := s.getCustomerEvent(ctx, id, ownerID); err != nil {
		return eventDto.EventRes{}, err
	}
	return s.ResetTemplate(ctx, id, kind)
}

func (s *EventService) UpdateCustomerEvent(ctx context.Context, id, ownerID string, req eventDto.UpdateEventReq) (eventDto.EventRes, error) {
	if _, err := s.getCustomerEvent(ctx, id, ownerID); err != nil {
		return eventDto.EventRes{}, err
	}
	return s.UpdateEvent(ctx, id, req)
}

func (s *EventService) PublishCustomerEvent(ctx context.Context, id, ownerID string) (eventDto.EventRes, error) {
	event, err := s.getCustomerEvent(ctx, id, ownerID)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	if event.BillingState != models.EventBillingActive {
		return eventDto.EventRes{}, ErrEventNotApproved
	}
	return s.setStatus(ctx, id, models.EventStatusLive)
}

func (s *EventService) ArchiveCustomerEvent(ctx context.Context, id, ownerID string) (eventDto.EventRes, error) {
	if _, err := s.getCustomerEvent(ctx, id, ownerID); err != nil {
		return eventDto.EventRes{}, err
	}
	return s.setStatus(ctx, id, models.EventStatusArchived)
}

func (s *EventService) RotateCustomerDisplayToken(ctx context.Context, id, ownerID string) (eventDto.RotateDisplayTokenRes, error) {
	if _, err := s.getCustomerEvent(ctx, id, ownerID); err != nil {
		return eventDto.RotateDisplayTokenRes{}, err
	}
	return s.RotateDisplayToken(ctx, id)
}

func (s *EventService) UpdatePlan(ctx context.Context, id string, req eventDto.UpdatePlanReq, plan *models.EventPlan) (eventDto.EventRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	state := models.EventBillingState(req.BillingState)
	switch state {
	case models.EventBillingDraft, models.EventBillingPendingReview, models.EventBillingActive, models.EventBillingRejected, models.EventBillingExpired:
	default:
		return eventDto.EventRes{}, ErrEventNotApproved
	}
	if plan != nil {
		event.PlanCode = plan.Code
		event.PlanSnapshot = plan.Snapshot()
		event.RetentionDays = plan.RetentionDays
	}
	if req.PhotoQuota != nil {
		event.PlanSnapshot.PhotoQuota = *req.PhotoQuota
	}
	if req.BoomerangQuota != nil {
		event.PlanSnapshot.BoomerangQuota = *req.BoomerangQuota
	}
	if req.RetentionDays != nil {
		event.PlanSnapshot.RetentionDays = *req.RetentionDays
		event.RetentionDays = *req.RetentionDays
	}
	event.BillingState = state
	if state != models.EventBillingActive {
		// A pending, rejected or expired plan must not leave a public booth or display running.
		event.Status = models.EventStatusDraft
	}
	event.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, event); err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) ConsumeMediaQuota(ctx context.Context, eventID string, kind models.MediaKind, allowOverQuota bool) error {
	ok, err := s.repo.ConsumeQuota(ctx, eventID, kind, allowOverQuota)
	if err != nil {
		return err
	}
	if !ok {
		return ErrEventQuotaExceeded
	}
	return nil
}

func (s *EventService) ReleaseMediaQuota(ctx context.Context, eventID string, kind models.MediaKind) {
	_ = s.repo.ReleaseQuota(ctx, eventID, kind)
}

func (s *EventService) GetPublicEventBySlug(ctx context.Context, slug string) (eventDto.EventRes, error) {
	event, err := s.repo.FindBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return eventDto.EventRes{}, ErrEventNotFound
		}
		return eventDto.EventRes{}, err
	}
	if !event.IsLiveAt(time.Now()) {
		return eventDto.EventRes{}, ErrEventNotLive
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) GetEventByDisplayToken(ctx context.Context, rawToken string) (eventDto.EventRes, models.Event, error) {
	event, err := s.repo.FindByDisplayTokenHash(ctx, identity.HashToken(rawToken))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return eventDto.EventRes{}, models.Event{}, ErrEventNotFound
		}
		return eventDto.EventRes{}, models.Event{}, err
	}
	if event.Status != models.EventStatusLive {
		return eventDto.EventRes{}, models.Event{}, ErrEventNotLive
	}
	return eventDto.NewEventRes(event), event, nil
}

func (s *EventService) setStatus(ctx context.Context, id string, status models.EventStatus) (eventDto.EventRes, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return eventDto.EventRes{}, err
	}
	event.Status = status
	event.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, event); err != nil {
		return eventDto.EventRes{}, err
	}
	return eventDto.NewEventRes(event), nil
}

func (s *EventService) getCustomerEvent(ctx context.Context, id, ownerID string) (models.Event, error) {
	event, err := s.getEvent(ctx, id)
	if err != nil {
		return models.Event{}, err
	}
	if event.OwnerType != models.EventOwnerCustomer || event.OwnerID != ownerID {
		return models.Event{}, ErrEventNotFound
	}
	return event, nil
}

func (s *EventService) getEvent(ctx context.Context, id string) (models.Event, error) {
	event, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.Event{}, ErrEventNotFound
		}
		return models.Event{}, err
	}
	return event, nil
}

func eventIDSuffix(id primitive.ObjectID) string {
	hex := id.Hex()
	if len(hex) <= 6 {
		return hex
	}
	return hex[len(hex)-6:]
}
