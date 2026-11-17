package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CustomerStatus string

const (
	CustomerStatusActive   CustomerStatus = "active"
	CustomerStatusDisabled CustomerStatus = "disabled"
)

type Customer struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email        string             `bson:"email" json:"email"`
	PasswordHash string             `bson:"passwordHash" json:"-"`
	Status       CustomerStatus     `bson:"status" json:"status"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type CustomerSession struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	CustomerID primitive.ObjectID `bson:"customerId" json:"customerId"`
	TokenHash  string             `bson:"tokenHash" json:"-"`
	CSRFHash   string             `bson:"csrfHash" json:"-"`
	ExpiresAt  time.Time          `bson:"expiresAt" json:"expiresAt"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
}

type EventPlan struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code           string             `bson:"code" json:"code"`
	Name           string             `bson:"name" json:"name"`
	Description    string             `bson:"description,omitempty" json:"description,omitempty"`
	Price          int64              `bson:"price" json:"price"`
	Currency       string             `bson:"currency" json:"currency"`
	PhotoQuota     int                `bson:"photoQuota" json:"photoQuota"`
	BoomerangQuota int                `bson:"boomerangQuota" json:"boomerangQuota"`
	RetentionDays  int                `bson:"retentionDays" json:"retentionDays"`
	Active         bool               `bson:"active" json:"active"`
	SortOrder      int                `bson:"sortOrder" json:"sortOrder"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type EventPlanSnapshot struct {
	Code           string `bson:"code" json:"code"`
	Name           string `bson:"name" json:"name"`
	Price          int64  `bson:"price" json:"price"`
	Currency       string `bson:"currency" json:"currency"`
	PhotoQuota     int    `bson:"photoQuota" json:"photoQuota"`
	BoomerangQuota int    `bson:"boomerangQuota" json:"boomerangQuota"`
	RetentionDays  int    `bson:"retentionDays" json:"retentionDays"`
}

func (p EventPlan) Snapshot() EventPlanSnapshot {
	return EventPlanSnapshot{
		Code:           p.Code,
		Name:           p.Name,
		Price:          p.Price,
		Currency:       p.Currency,
		PhotoQuota:     p.PhotoQuota,
		BoomerangQuota: p.BoomerangQuota,
		RetentionDays:  p.RetentionDays,
	}
}
