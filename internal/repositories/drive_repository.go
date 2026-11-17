package repositories

import (
	"context"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type DriveRepository interface {
	EnsureIndexes(ctx context.Context) error

	UpsertConnection(ctx context.Context, conn models.DriveConnection) (string, error)
	FindConnection(ctx context.Context, id string) (models.DriveConnection, error)
	FindOperatorConnection(ctx context.Context) (models.DriveConnection, error)
	FindCustomerConnection(ctx context.Context, customerID string) (models.DriveConnection, error)

	CreateOAuthState(ctx context.Context, state models.DriveOAuthState) error
	ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (models.DriveOAuthState, error)

	CreateSetupToken(ctx context.Context, token models.DriveSetupToken) error
	FindSetupToken(ctx context.Context, tokenHash string) (models.DriveSetupToken, error)
	MarkSetupTokenUsed(ctx context.Context, tokenHash string, now time.Time) error
	SetSetupConnection(ctx context.Context, tokenHash, connectionID string) error

	UpsertDestination(ctx context.Context, destination models.DriveDestination) error
	FindDestinationByEvent(ctx context.Context, eventID string) (models.DriveDestination, error)

	FindOrCreateGuestMapping(ctx context.Context, mapping models.DriveGuestMapping) (models.DriveGuestMapping, error)
	SetGuestFolder(ctx context.Context, eventID, guestIDHash, folderID string) error

	InsertExportJobIfMissing(ctx context.Context, job models.DriveExportJob) error
	ClaimExportJob(ctx context.Context, now time.Time, lease time.Duration) (models.DriveExportJob, bool, error)
	CompleteExportJob(ctx context.Context, id, mediaFileID, messageFileID string) error
	FailExportJob(ctx context.Context, id, code string) error
	RescheduleExportJob(ctx context.Context, id, code string) error
	RetryFailedByEvent(ctx context.Context, eventID string) (int64, error)
	ResetExportsByEvent(ctx context.Context, eventID string) (int64, error)
	SetExportJobGuestHash(ctx context.Context, mediaID, guestHash string) error
	SetDestinationEventFolder(ctx context.Context, eventID, folderID string) error
	DeleteByMediaID(ctx context.Context, mediaID string) error
	ExportSummaryByEvent(ctx context.Context, eventID string) (map[string]int64, error)
}
