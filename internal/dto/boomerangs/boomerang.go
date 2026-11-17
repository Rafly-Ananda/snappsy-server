package boomerangs

import "time"

type CreateIntentReq struct {
	EventID  string `json:"eventId" binding:"required"`
	MimeType string `json:"mimeType" binding:"required"`
	Bytes    int64  `json:"bytes"`
}

type CreateIntentRes struct {
	IntentID    string    `json:"intentId"`
	StatusToken string    `json:"statusToken"`
	SourceUrl   string    `json:"sourceUrl"`
	SourceKey   string    `json:"sourceKey"`
	ExpiresAt   time.Time `json:"expiresAt"`
	MaxSeconds  int       `json:"maxSeconds"`
}

type QueueStatusRes struct {
	Active int64 `json:"active"`
	Failed int64 `json:"failed"`
}

type CommitReq struct {
	IntentID       string  `json:"intentId" binding:"required"`
	StatusToken    string  `json:"statusToken" binding:"required"`
	Username       string  `json:"username" binding:"required"`
	GuestID        string  `json:"guestId"`
	Captions       string  `json:"captions"`
	Consent        bool    `json:"consent"`
	StartSeconds   float64 `json:"startSeconds"`
	SegmentSeconds float64 `json:"segmentSeconds"`
}
