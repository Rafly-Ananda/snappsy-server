package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MediaJobState string

const (
	MediaJobQueued     MediaJobState = "queued"
	MediaJobProcessing MediaJobState = "processing"
	MediaJobReady      MediaJobState = "ready"
	MediaJobFailed     MediaJobState = "failed"
)

type MediaJob struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MediaID        string             `bson:"mediaId" json:"mediaId"`
	EventID        string             `bson:"eventId" json:"eventId"`
	SourceKey      string             `bson:"sourceKey" json:"sourceKey"`
	State          MediaJobState      `bson:"state" json:"state"`
	StartSeconds   float64            `bson:"startSeconds" json:"startSeconds"`
	SegmentSeconds float64            `bson:"segmentSeconds" json:"segmentSeconds"`
	Attempts       int                `bson:"attempts" json:"attempts"`
	LeaseUntil     *time.Time         `bson:"leaseUntil,omitempty" json:"leaseUntil,omitempty"`
	ErrorCode      string             `bson:"errorCode,omitempty" json:"errorCode,omitempty"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
}
