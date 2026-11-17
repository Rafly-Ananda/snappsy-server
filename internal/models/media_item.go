package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MediaKind string

const (
	MediaKindPhoto      MediaKind = "photo"
	MediaKindPhotoStrip MediaKind = "photo_strip"
	MediaKindBoomerang  MediaKind = "boomerang"
)

type MediaProcessingState string

const (
	MediaProcessingReady      MediaProcessingState = "ready"
	MediaProcessingProcessing MediaProcessingState = "processing"
	MediaProcessingFailed     MediaProcessingState = "failed"
)

type MediaModerationState string

const (
	MediaModerationPending  MediaModerationState = "pending"
	MediaModerationApproved MediaModerationState = "approved"
	MediaModerationHidden   MediaModerationState = "hidden"
)

type MediaItem struct {
	ID              primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	EventID         string               `bson:"eventId" json:"eventId"`
	Kind            MediaKind            `bson:"kind" json:"kind"`
	Username        string               `bson:"username" json:"username"`
	GuestIDHash     string               `bson:"guestIdHash,omitempty" json:"-"`
	ObjectKey       string               `bson:"objectKey,omitempty" json:"objectKey,omitempty"`
	BaseKey         string               `bson:"baseKey,omitempty" json:"baseKey,omitempty"`
	CompositeKey    string               `bson:"compositeKey,omitempty" json:"compositeKey,omitempty"`
	ThumbnailKey    string               `bson:"thumbnailKey,omitempty" json:"thumbnailKey,omitempty"`
	SourceKey       string               `bson:"sourceKey,omitempty" json:"sourceKey,omitempty"`
	OutputKey       string               `bson:"outputKey,omitempty" json:"outputKey,omitempty"`
	PosterKey       string               `bson:"posterKey,omitempty" json:"posterKey,omitempty"`
	Captions        string               `bson:"captions" json:"captions"`
	Orientation     string               `bson:"orientation" json:"orientation"`
	ProcessingState MediaProcessingState `bson:"processingState,omitempty" json:"processingState"`
	ModerationState MediaModerationState `bson:"moderationState,omitempty" json:"moderationState"`
	TemplateID      string               `bson:"templateId,omitempty" json:"templateId,omitempty"`
	TemplateVersion int                  `bson:"templateVersion,omitempty" json:"templateVersion,omitempty"`
	GuestEditState  string               `bson:"guestEditState,omitempty" json:"guestEditState,omitempty"`
	ConsentAt       *time.Time           `bson:"consentAt,omitempty" json:"consentAt,omitempty"`
	RetentionAt     *time.Time           `bson:"retentionAt,omitempty" json:"retentionAt,omitempty"`
	StatusTokenHash string               `bson:"statusTokenHash,omitempty" json:"-"`
	UploadIntentID  string               `bson:"uploadIntentId,omitempty" json:"uploadIntentId,omitempty"`
	CreatedAt       time.Time            `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time            `bson:"updatedAt" json:"updatedAt"`
}

// Images is kept as a compatibility alias while old wedding records are migrated.
type Images = MediaItem
