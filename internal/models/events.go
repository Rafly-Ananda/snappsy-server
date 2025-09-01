package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Events struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	EventName   string             `bson:"eventName" json:"eventNmae"`
	Description string             `bson:"description" json:"description"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}
