package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AdminSession struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TokenHash string             `bson:"tokenHash" json:"-"`
	CSRFHash  string             `bson:"csrfHash" json:"-"`
	ExpiresAt time.Time          `bson:"expiresAt" json:"expiresAt"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
}
