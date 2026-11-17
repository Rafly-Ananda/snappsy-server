package events

import (
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type CreateEventReq struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type CreateCustomerEventReq struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	PlanCode        string `json:"planCode" binding:"required"`
	ContactEmail    string `json:"contactEmail" binding:"required,email"`
	ContactWhatsApp string `json:"contactWhatsApp" binding:"required"`
}

type UpdateEventReq struct {
	Name                     *string                   `json:"name"`
	Description              *string                   `json:"description"`
	ContactEmail             *string                   `json:"contactEmail"`
	ContactWhatsApp          *string                   `json:"contactWhatsApp"`
	CaptureModes             *models.EventCaptureModes `json:"captureModes"`
	Countdowns               *models.EventCountdowns   `json:"countdowns"`
	StartsAt                 *time.Time                `json:"startsAt"`
	EndsAt                   *time.Time                `json:"endsAt"`
	Branding                 *models.EventBranding     `json:"branding"`
	UploadPolicy             *models.EventUploadPolicy `json:"uploadPolicy"`
	RetentionDays            *int                      `json:"retentionDays"`
	DesignVersion            *int                      `json:"designVersion"`
	TemplateID               *string                   `json:"templateId"`
	TemplateVersion          *int                      `json:"templateVersion"`
	StripTemplateID          *string                   `json:"stripTemplateId"`
	StripTemplateVersion     *int                      `json:"stripTemplateVersion"`
	BoomerangTemplateID      *string                   `json:"boomerangTemplateId"`
	BoomerangTemplateVersion *int                      `json:"boomerangTemplateVersion"`
	StickerAssetIDs          *[]string                 `json:"stickerAssetIds"`
	HiddenStickerIDs         *[]string                 `json:"hiddenStickerIds"`
	AllowedTemplateIDs       *[]string                 `json:"allowedTemplateIds"`
}

type EventRes struct {
	ID                       string                   `json:"id"`
	OwnerType                models.EventOwnerType    `json:"ownerType"`
	OwnerID                  string                   `json:"ownerId,omitempty"`
	BillingState             models.EventBillingState `json:"billingState"`
	PlanCode                 string                   `json:"planCode,omitempty"`
	Plan                     models.EventPlanSnapshot `json:"plan"`
	ContactEmail             string                   `json:"contactEmail,omitempty"`
	ContactWhatsApp          string                   `json:"contactWhatsApp,omitempty"`
	CaptureModes             models.EventCaptureModes `json:"captureModes"`
	Countdowns               models.EventCountdowns   `json:"countdowns"`
	UsedPhotos               int                      `json:"usedPhotos"`
	UsedBoomerangs           int                      `json:"usedBoomerangs"`
	Name                     string                   `json:"name"`
	Description              string                   `json:"description"`
	Slug                     string                   `json:"slug"`
	Status                   models.EventStatus       `json:"status"`
	StartsAt                 *time.Time               `json:"startsAt,omitempty"`
	EndsAt                   *time.Time               `json:"endsAt,omitempty"`
	Branding                 models.EventBranding     `json:"branding"`
	UploadPolicy             models.EventUploadPolicy `json:"uploadPolicy"`
	RetentionDays            int                      `json:"retentionDays"`
	DesignVersion            int                      `json:"designVersion"`
	TemplateID               string                   `json:"templateId,omitempty"`
	TemplateVersion          int                      `json:"templateVersion,omitempty"`
	StripTemplateID          string                   `json:"stripTemplateId,omitempty"`
	StripTemplateVersion     int                      `json:"stripTemplateVersion,omitempty"`
	BoomerangTemplateID      string                   `json:"boomerangTemplateId,omitempty"`
	BoomerangTemplateVersion int                      `json:"boomerangTemplateVersion,omitempty"`
	StickerAssetIDs          []string                 `json:"stickerAssetIds,omitempty"`
	HiddenStickerIDs         []string                 `json:"hiddenStickerIds,omitempty"`
	AllowedTemplateIDs       []string                 `json:"allowedTemplateIds,omitempty"`
	CreatedAt                time.Time                `json:"createdAt"`
	UpdatedAt                time.Time                `json:"updatedAt"`
}

type CreateEventRes struct {
	EventRes
	DisplayToken string `json:"displayToken,omitempty"`
}

type RotateDisplayTokenRes struct {
	DisplayToken string `json:"displayToken"`
}

type UpdatePlanReq struct {
	BillingState   string `json:"billingState" binding:"required"`
	PlanCode       string `json:"planCode"`
	PhotoQuota     *int   `json:"photoQuota"`
	BoomerangQuota *int   `json:"boomerangQuota"`
	RetentionDays  *int   `json:"retentionDays"`
}

func NewEventRes(event models.Event) EventRes {
	return EventRes{
		ID:                       event.ID.Hex(),
		OwnerType:                event.OwnerType,
		OwnerID:                  event.OwnerID,
		BillingState:             event.BillingState,
		PlanCode:                 event.PlanCode,
		Plan:                     event.PlanSnapshot,
		ContactEmail:             event.ContactEmail,
		ContactWhatsApp:          event.ContactWhatsApp,
		CaptureModes:             event.CaptureModes,
		Countdowns:               event.Countdowns.Normalized(),
		UsedPhotos:               event.UsedPhotos,
		UsedBoomerangs:           event.UsedBoomerangs,
		Name:                     event.Name,
		Description:              event.Description,
		Slug:                     event.Slug,
		Status:                   event.Status,
		StartsAt:                 event.StartsAt,
		EndsAt:                   event.EndsAt,
		Branding:                 event.Branding,
		UploadPolicy:             event.UploadPolicy,
		RetentionDays:            event.RetentionDays,
		DesignVersion:            event.DesignVersion,
		TemplateID:               event.TemplateID,
		TemplateVersion:          event.TemplateVersion,
		StripTemplateID:          event.StripTemplateID,
		StripTemplateVersion:     event.StripTemplateVersion,
		BoomerangTemplateID:      event.BoomerangTemplateID,
		BoomerangTemplateVersion: event.BoomerangTemplateVersion,
		StickerAssetIDs:          event.StickerAssetIDs,
		HiddenStickerIDs:         event.HiddenStickerIDs,
		AllowedTemplateIDs:       event.AllowedTemplateIDs,
		CreatedAt:                event.CreatedAt,
		UpdatedAt:                event.UpdatedAt,
	}
}
