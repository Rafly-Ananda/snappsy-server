package photos

import (
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type CreateIntentReq struct {
	EventID  string `json:"eventId" binding:"required"`
	MimeType string `json:"mimeType" binding:"required"`
}

type CreateIntentRes struct {
	IntentID     string    `json:"intentId"`
	StatusToken  string    `json:"statusToken"`
	BaseUrl      string    `json:"baseUrl"`
	CompositeUrl string    `json:"compositeUrl"`
	ThumbnailUrl string    `json:"thumbnailUrl"`
	BaseKey      string    `json:"baseKey"`
	CompositeKey string    `json:"compositeKey"`
	ThumbnailKey string    `json:"thumbnailKey"`
	ExpiresAt    time.Time `json:"expiresAt"`
}

type CommitReq struct {
	IntentID        string `json:"intentId" binding:"required"`
	StatusToken     string `json:"statusToken" binding:"required"`
	Username        string `json:"username" binding:"required"`
	GuestID         string `json:"guestId"`
	Captions        string `json:"captions"`
	Orientation     string `json:"orientation" binding:"required"`
	Kind            string `json:"kind"`
	TemplateID      string `json:"templateId"`
	TemplateVersion int    `json:"templateVersion"`
	GuestEditState  string `json:"guestEditState"`
	Consent         bool   `json:"consent"`
}

type CommitRes struct {
	ID              string                      `json:"id"`
	ProcessingState models.MediaProcessingState `json:"processingState"`
	ModerationState models.MediaModerationState `json:"moderationState"`
}

type StatusRes struct {
	ID              string                      `json:"id"`
	Kind            models.MediaKind            `json:"kind"`
	ProcessingState models.MediaProcessingState `json:"processingState"`
	ModerationState models.MediaModerationState `json:"moderationState"`
	URL             string                      `json:"url,omitempty"`
	PlaybackURL     string                      `json:"playbackUrl,omitempty"`
	DownloadURL     string                      `json:"downloadUrl,omitempty"`
}

type ModerationReq struct {
	State string `json:"state" binding:"required"`
}

type AdminMediaListRes struct {
	Data       []AdminMediaRes `json:"data"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type AdminMediaRes struct {
	ID              string                      `json:"id"`
	Kind            models.MediaKind            `json:"kind"`
	Username        string                      `json:"username"`
	Captions        string                      `json:"captions"`
	ProcessingState models.MediaProcessingState `json:"processingState"`
	ModerationState models.MediaModerationState `json:"moderationState"`
	URL             string                      `json:"url,omitempty"`
	PlaybackURL     string                      `json:"playbackUrl,omitempty"`
	CreatedAt       time.Time                   `json:"createdAt"`
}

type DesignAsset struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	ObjectKey string `json:"objectKey"`
	URL       string `json:"url"`
}

type DesignRes struct {
	EventID         string                `json:"eventId"`
	TemplateID      string                `json:"templateId"`
	TemplateVersion int                   `json:"templateVersion"`
	Schema          models.TemplateSchema `json:"schema"`
	StripSchema     models.TemplateSchema `json:"stripSchema,omitempty"`
	BoomerangSchema models.TemplateSchema `json:"boomerangSchema,omitempty"`
	StripStyle      models.TemplateStyle  `json:"stripStyle,omitempty"`
	BoomerangStyle  models.TemplateStyle  `json:"boomerangStyle,omitempty"`
	Assets          []DesignAsset         `json:"assets"`
}
