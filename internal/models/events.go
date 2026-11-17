package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventStatus string

const (
	EventStatusDraft    EventStatus = "draft"
	EventStatusLive     EventStatus = "live"
	EventStatusArchived EventStatus = "archived"
)

type EventOwnerType string

const (
	EventOwnerPlatform EventOwnerType = "platform"
	EventOwnerCustomer EventOwnerType = "customer"
)

type EventBillingState string

const (
	EventBillingDraft         EventBillingState = "draft"
	EventBillingPendingReview EventBillingState = "pending_review"
	EventBillingActive        EventBillingState = "active"
	EventBillingRejected      EventBillingState = "rejected"
	EventBillingExpired       EventBillingState = "expired"
)

type EventCaptureModes struct {
	Photo     bool `bson:"photo" json:"photo"`
	Strip     bool `bson:"strip" json:"strip"`
	Boomerang bool `bson:"boomerang" json:"boomerang"`
}

type EventCountdowns struct {
	Photo     int `bson:"photo" json:"photo"`
	Strip     int `bson:"strip" json:"strip"`
	Boomerang int `bson:"boomerang" json:"boomerang"`
}

func (c EventCountdowns) Normalized() EventCountdowns {
	clamp := func(value int) int {
		if value < 0 {
			return 0
		}
		if value > 10 {
			return 10
		}
		return value
	}
	return EventCountdowns{Photo: clamp(c.Photo), Strip: clamp(c.Strip), Boomerang: clamp(c.Boomerang)}
}

type EventBranding struct {
	Headline       string `bson:"headline,omitempty" json:"headline,omitempty"`
	Subline        string `bson:"subline,omitempty" json:"subline,omitempty"`
	LogoObjectKey  string `bson:"logoObjectKey,omitempty" json:"logoObjectKey,omitempty"`
	PrimaryColor   string `bson:"primaryColor,omitempty" json:"primaryColor,omitempty"`
	SecondaryColor string `bson:"secondaryColor,omitempty" json:"secondaryColor,omitempty"`
}

type EventUploadPolicy struct {
	AcceptUploads   bool `bson:"acceptUploads" json:"acceptUploads"`
	RequireApproval bool `bson:"requireApproval" json:"requireApproval"`
	AllowDownload   bool `bson:"allowDownload" json:"allowDownload"`
	AllowOverQuota  bool `bson:"allowOverQuota,omitempty" json:"allowOverQuota"`
}

type Event struct {
	ID                       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerType                EventOwnerType     `bson:"ownerType" json:"ownerType"`
	OwnerID                  string             `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	BillingState             EventBillingState  `bson:"billingState" json:"billingState"`
	PlanCode                 string             `bson:"planCode,omitempty" json:"planCode,omitempty"`
	PlanSnapshot             EventPlanSnapshot  `bson:"planSnapshot" json:"planSnapshot"`
	ContactEmail             string             `bson:"contactEmail,omitempty" json:"contactEmail,omitempty"`
	ContactWhatsApp          string             `bson:"contactWhatsApp,omitempty" json:"contactWhatsApp,omitempty"`
	CaptureModes             EventCaptureModes  `bson:"captureModes" json:"captureModes"`
	Countdowns               EventCountdowns    `bson:"countdowns" json:"countdowns"`
	UsedPhotos               int                `bson:"usedPhotos" json:"usedPhotos"`
	UsedBoomerangs           int                `bson:"usedBoomerangs" json:"usedBoomerangs"`
	Name                     string             `bson:"name" json:"name"`
	Description              string             `bson:"description" json:"description"`
	Slug                     string             `bson:"slug" json:"slug"`
	Status                   EventStatus        `bson:"status" json:"status"`
	StartsAt                 *time.Time         `bson:"startsAt,omitempty" json:"startsAt,omitempty"`
	EndsAt                   *time.Time         `bson:"endsAt,omitempty" json:"endsAt,omitempty"`
	Branding                 EventBranding      `bson:"branding" json:"branding"`
	UploadPolicy             EventUploadPolicy  `bson:"uploadPolicy" json:"uploadPolicy"`
	RetentionDays            int                `bson:"retentionDays" json:"retentionDays"`
	DesignVersion            int                `bson:"designVersion,omitempty" json:"designVersion"`
	TemplateID               string             `bson:"templateId,omitempty" json:"templateId,omitempty"`
	TemplateVersion          int                `bson:"templateVersion,omitempty" json:"templateVersion,omitempty"`
	StripTemplateID          string             `bson:"stripTemplateId,omitempty" json:"stripTemplateId,omitempty"`
	StripTemplateVersion     int                `bson:"stripTemplateVersion,omitempty" json:"stripTemplateVersion,omitempty"`
	BoomerangTemplateID      string             `bson:"boomerangTemplateId,omitempty" json:"boomerangTemplateId,omitempty"`
	BoomerangTemplateVersion int                `bson:"boomerangTemplateVersion,omitempty" json:"boomerangTemplateVersion,omitempty"`
	StickerAssetIDs          []string           `bson:"stickerAssetIds,omitempty" json:"stickerAssetIds,omitempty"`
	HiddenStickerIDs         []string           `bson:"hiddenStickerIds,omitempty" json:"hiddenStickerIds,omitempty"`
	AllowedTemplateIDs       []string           `bson:"allowedTemplateIds,omitempty" json:"allowedTemplateIds,omitempty"`
	DisplayTokenHash         string             `bson:"displayTokenHash,omitempty" json:"-"`
	CreatedAt                time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt                time.Time          `bson:"updatedAt" json:"updatedAt"`
}

func (e Event) AllowsCapture(kind MediaKind) bool {
	switch kind {
	case MediaKindPhoto:
		return e.CaptureModes.Photo
	case MediaKindPhotoStrip:
		return e.CaptureModes.Strip
	case MediaKindBoomerang:
		return e.CaptureModes.Boomerang
	default:
		return false
	}
}

func (e Event) IsLiveAt(now time.Time) bool {
	if e.Status != EventStatusLive {
		return false
	}
	if e.StartsAt != nil && now.Before(*e.StartsAt) {
		return false
	}
	if e.EndsAt != nil && now.After(*e.EndsAt) {
		return false
	}
	return true
}
