package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TemplateLayerType string

const (
	TemplateLayerImage TemplateLayerType = "image"
	TemplateLayerText  TemplateLayerType = "text"
)

type TemplateKind string

const (
	TemplateKindPhoto      TemplateKind = "photo"
	TemplateKindStrip      TemplateKind = "strip"
	TemplateKindVideoFrame TemplateKind = "video_frame"
)

type TemplateStyle struct {
	Preset string `bson:"preset,omitempty" json:"preset,omitempty"`
	Paper  string `bson:"paper,omitempty" json:"paper,omitempty"`
	Ink    string `bson:"ink,omitempty" json:"ink,omitempty"`
	Accent string `bson:"accent,omitempty" json:"accent,omitempty"`
	Title  string `bson:"title,omitempty" json:"title,omitempty"`
}

type TemplateCanvas struct {
	Width      int    `bson:"width" json:"width"`
	Height     int    `bson:"height" json:"height"`
	Background string `bson:"background" json:"background"`
}

type TemplatePhotoSlot struct {
	X        float64 `bson:"x" json:"x"`
	Y        float64 `bson:"y" json:"y"`
	Width    float64 `bson:"width" json:"width"`
	Height   float64 `bson:"height" json:"height"`
	Rotation float64 `bson:"rotation" json:"rotation"`
}

type TemplateLayer struct {
	ID       string            `bson:"id" json:"id"`
	Type     TemplateLayerType `bson:"type" json:"type"`
	AssetID  string            `bson:"assetId,omitempty" json:"assetId,omitempty"`
	Text     string            `bson:"text,omitempty" json:"text,omitempty"`
	X        float64           `bson:"x" json:"x"`
	Y        float64           `bson:"y" json:"y"`
	Width    float64           `bson:"width" json:"width"`
	Height   float64           `bson:"height" json:"height"`
	Rotation float64           `bson:"rotation" json:"rotation"`
	Locked   bool              `bson:"locked" json:"locked"`
}

type TemplateSchema struct {
	Canvas     TemplateCanvas      `bson:"canvas" json:"canvas"`
	PhotoSlot  TemplatePhotoSlot   `bson:"photoSlot" json:"photoSlot"`
	PhotoSlots []TemplatePhotoSlot `bson:"photoSlots,omitempty" json:"photoSlots,omitempty"`
	Layers     []TemplateLayer     `bson:"layers" json:"layers"`
	Style      TemplateStyle       `bson:"style,omitempty" json:"style,omitempty"`
}

type Template struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Kind               TemplateKind       `bson:"kind,omitempty" json:"kind"`
	OwnerType          string             `bson:"ownerType,omitempty" json:"ownerType,omitempty"`
	OwnerID            string             `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	EventID            string             `bson:"eventId,omitempty" json:"eventId,omitempty"`
	Name               string             `bson:"name" json:"name"`
	CurrentDraftVer    int                `bson:"currentDraftVersion" json:"currentDraftVersion"`
	LatestPublishedVer int                `bson:"latestPublishedVersion" json:"latestPublishedVersion"`
	CreatedAt          time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type TemplateVersion struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TemplateID string             `bson:"templateId" json:"templateId"`
	Version    int                `bson:"version" json:"version"`
	Status     string             `bson:"status" json:"status"`
	Schema     TemplateSchema     `bson:"schema" json:"schema"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
}
