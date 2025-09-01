package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Images struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	EventId   string             `bson:"eventId" json:"sessionId"`
	Username  string             `bson:"username" json:"username"`
	MinioKey  string             `bson:"minioKey" json:"minioKey"`
	Captions  string             `bson:"captions" json:"captions"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}
