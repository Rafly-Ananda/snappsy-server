package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AssetKind string

const (
	AssetKindSticker    AssetKind = "sticker"
	AssetKindBackground AssetKind = "background"
	AssetKindLogo       AssetKind = "logo"
	AssetKindTexture    AssetKind = "texture"
	AssetKindTemplate   AssetKind = "template_image"
)

type Asset struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	OwnerType   string             `bson:"ownerType,omitempty" json:"ownerType,omitempty"`
	OwnerID     string             `bson:"ownerId,omitempty" json:"ownerId,omitempty"`
	EventID     string             `bson:"eventId,omitempty" json:"eventId,omitempty"`
	Disabled    bool               `bson:"disabled,omitempty" json:"disabled"`
	Kind        AssetKind          `bson:"kind" json:"kind"`
	Name        string             `bson:"name" json:"name"`
	ObjectKey   string             `bson:"objectKey" json:"objectKey"`
	MimeType    string             `bson:"mimeType" json:"mimeType"`
	License     string             `bson:"license" json:"license"`
	Source      string             `bson:"source" json:"source"`
	Attribution string             `bson:"attribution" json:"attribution"`
	Bytes       int64              `bson:"bytes" json:"bytes"`
	Width       int                `bson:"width" json:"width"`
	Height      int                `bson:"height" json:"height"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}
