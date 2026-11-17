package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type DriveConnectionKind string

const (
	DriveConnectionOperator DriveConnectionKind = "operator"
	DriveConnectionCustomer DriveConnectionKind = "customer"
)

type DriveConnectionStatus string

const (
	DriveConnectionActive   DriveConnectionStatus = "active"
	DriveConnectionRevoked  DriveConnectionStatus = "revoked"
	DriveConnectionDisabled DriveConnectionStatus = "disabled"
)

type DriveConnection struct {
	ID                    primitive.ObjectID    `bson:"_id,omitempty" json:"id"`
	Kind                  DriveConnectionKind   `bson:"kind" json:"kind"`
	CustomerID            string                `bson:"customerId,omitempty" json:"customerId,omitempty"`
	Email                 string                `bson:"email,omitempty" json:"email,omitempty"`
	EncryptedRefreshToken string                `bson:"encryptedRefreshToken" json:"-"`
	Scope                 string                `bson:"scope,omitempty" json:"scope,omitempty"`
	Status                DriveConnectionStatus `bson:"status" json:"status"`
	CreatedAt             time.Time             `bson:"createdAt" json:"createdAt"`
	UpdatedAt             time.Time             `bson:"updatedAt" json:"updatedAt"`
}

type DriveOAuthState struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	StateHash      string             `bson:"stateHash" json:"-"`
	Kind           string             `bson:"kind" json:"kind"`
	CustomerID     string             `bson:"customerId,omitempty" json:"customerId,omitempty"`
	EventID        string             `bson:"eventId,omitempty" json:"eventId,omitempty"`
	SetupTokenHash string             `bson:"setupTokenHash,omitempty" json:"-"`
	SetupTokenEnc  string             `bson:"setupTokenEnc,omitempty" json:"-"`
	ExpiresAt      time.Time          `bson:"expiresAt" json:"expiresAt"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
}

type DriveSetupToken struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	TokenHash    string             `bson:"tokenHash" json:"-"`
	EventID      string             `bson:"eventId" json:"eventId"`
	ExpiresAt    time.Time          `bson:"expiresAt" json:"expiresAt"`
	ConnectionID string             `bson:"connectionId,omitempty" json:"connectionId,omitempty"`
	UsedAt       *time.Time         `bson:"usedAt,omitempty" json:"usedAt,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt" json:"createdAt"`
}

type DriveDestination struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID       string             `bson:"eventId" json:"eventId"`
	ConnectionID  string             `bson:"connectionId" json:"connectionId"`
	FolderID      string             `bson:"folderId" json:"folderId"`
	FolderName    string             `bson:"folderName,omitempty" json:"folderName,omitempty"`
	EventFolderID string             `bson:"eventFolderId,omitempty" json:"eventFolderId,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type DriveGuestMapping struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	EventID     string             `bson:"eventId" json:"eventId"`
	GuestIDHash string             `bson:"guestIdHash" json:"-"`
	Username    string             `bson:"username" json:"username"`
	FolderID    string             `bson:"folderId" json:"folderId"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type DriveExportState string

const (
	DriveExportPending   DriveExportState = "pending"
	DriveExportExporting DriveExportState = "exporting"
	DriveExportExported  DriveExportState = "exported"
	DriveExportFailed    DriveExportState = "failed"
)

type DriveExportJob struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	MediaID         string             `bson:"mediaId" json:"mediaId"`
	EventID         string             `bson:"eventId" json:"eventId"`
	GuestIDHash     string             `bson:"guestIdHash" json:"-"`
	Attempts        int                `bson:"attempts" json:"attempts"`
	State           DriveExportState   `bson:"state" json:"state"`
	LeaseUntil      *time.Time         `bson:"leaseUntil,omitempty" json:"leaseUntil,omitempty"`
	ErrorCode       string             `bson:"errorCode,omitempty" json:"errorCode,omitempty"`
	MediaFileID     string             `bson:"mediaFileId,omitempty" json:"mediaFileId,omitempty"`
	MessageFileID   string             `bson:"messageFileId,omitempty" json:"messageFileId,omitempty"`
	MediaFileName   string             `bson:"mediaFileName,omitempty" json:"mediaFileName,omitempty"`
	MessageFileName string             `bson:"messageFileName,omitempty" json:"messageFileName,omitempty"`
	CreatedAt       time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time          `bson:"updatedAt" json:"updatedAt"`
}
