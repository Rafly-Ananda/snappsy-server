package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UploadIntent struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID         string             `bson:"eventId" json:"eventId"`
	BaseKey         string             `bson:"baseKey" json:"baseKey"`
	CompositeKey    string             `bson:"compositeKey" json:"compositeKey"`
	ThumbnailKey    string             `bson:"thumbnailKey" json:"thumbnailKey"`
	StatusTokenHash string             `bson:"statusTokenHash" json:"-"`
	MediaID         string             `bson:"mediaId,omitempty" json:"mediaId,omitempty"`
	ExpiresAt       time.Time          `bson:"expiresAt" json:"expiresAt"`
	CreatedAt       time.Time          `bson:"createdAt" json:"createdAt"`
}
