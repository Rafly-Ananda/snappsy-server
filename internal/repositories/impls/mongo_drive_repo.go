package impls

import (
	"context"
	"fmt"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDriveRepository struct {
	connections  *mongo.Collection
	oauthStates  *mongo.Collection
	setupTokens  *mongo.Collection
	destinations *mongo.Collection
	guests       *mongo.Collection
	jobs         *mongo.Collection
}

func NewMongoDriveRepository(db *mongo.Database, prefix string) *MongoDriveRepository {
	return &MongoDriveRepository{
		connections:  db.Collection(prefix + "_connections"),
		oauthStates:  db.Collection(prefix + "_oauth_states"),
		setupTokens:  db.Collection(prefix + "_setup_tokens"),
		destinations: db.Collection(prefix + "_destinations"),
		guests:       db.Collection(prefix + "_guests"),
		jobs:         db.Collection(prefix + "_export_jobs"),
	}
}

func (r *MongoDriveRepository) EnsureIndexes(ctx context.Context) error {
	connectionIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("idx_drive_connections_kind_status")},
		{Keys: bson.D{{Key: "kind", Value: 1}, {Key: "customerId", Value: 1}, {Key: "status", Value: 1}}, Options: options.Index().SetName("idx_drive_connections_customer")},
	}
	if _, err := r.connections.Indexes().CreateMany(ctx, connectionIndexes); err != nil {
		return fmt.Errorf("create drive connection indexes: %w", err)
	}
	stateIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "stateHash", Value: 1}}, Options: options.Index().SetName("idx_drive_oauth_state_hash").SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("idx_drive_oauth_state_expiry").SetExpireAfterSeconds(0)},
	}
	if _, err := r.oauthStates.Indexes().CreateMany(ctx, stateIndexes); err != nil {
		return fmt.Errorf("create drive oauth indexes: %w", err)
	}
	setupIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "tokenHash", Value: 1}}, Options: options.Index().SetName("idx_drive_setup_token_hash").SetUnique(true)},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetName("idx_drive_setup_expiry").SetExpireAfterSeconds(0)},
	}
	if _, err := r.setupTokens.Indexes().CreateMany(ctx, setupIndexes); err != nil {
		return fmt.Errorf("create drive setup indexes: %w", err)
	}
	destinationIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "eventId", Value: 1}}, Options: options.Index().SetName("idx_drive_destination_event").SetUnique(true)},
	}
	if _, err := r.destinations.Indexes().CreateMany(ctx, destinationIndexes); err != nil {
		return fmt.Errorf("create drive destination indexes: %w", err)
	}
	guestIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "guestIdHash", Value: 1}}, Options: options.Index().SetName("idx_drive_guest_event_hash").SetUnique(true)},
	}
	if _, err := r.guests.Indexes().CreateMany(ctx, guestIndexes); err != nil {
		return fmt.Errorf("create drive guest indexes: %w", err)
	}
	jobIndexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "mediaId", Value: 1}}, Options: options.Index().SetName("idx_drive_export_media").SetUnique(true)},
		{Keys: bson.D{{Key: "state", Value: 1}, {Key: "createdAt", Value: 1}}, Options: options.Index().SetName("idx_drive_export_state_created")},
		{Keys: bson.D{{Key: "eventId", Value: 1}, {Key: "state", Value: 1}}, Options: options.Index().SetName("idx_drive_export_event_state")},
	}
	if _, err := r.jobs.Indexes().CreateMany(ctx, jobIndexes); err != nil {
		return fmt.Errorf("create drive export indexes: %w", err)
	}
	return nil
}

func (r *MongoDriveRepository) UpsertConnection(ctx context.Context, conn models.DriveConnection) (string, error) {
	now := time.Now()
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = now
	}
	conn.UpdatedAt = now
	if conn.Status == "" {
		conn.Status = models.DriveConnectionActive
	}
	if conn.Kind == models.DriveConnectionOperator || (conn.Kind == models.DriveConnectionCustomer && conn.CustomerID != "") {
		filter := bson.M{"kind": conn.Kind}
		if conn.CustomerID != "" {
			filter["customerId"] = conn.CustomerID
		}
		update := bson.M{"$set": bson.M{
			"kind":                  conn.Kind,
			"customerId":            conn.CustomerID,
			"email":                 conn.Email,
			"encryptedRefreshToken": conn.EncryptedRefreshToken,
			"scope":                 conn.Scope,
			"status":                conn.Status,
			"updatedAt":             now,
		}, "$setOnInsert": bson.M{"createdAt": now}}
		opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
		var result models.DriveConnection
		if err := r.connections.FindOneAndUpdate(ctx, filter, update, opts).Decode(&result); err != nil {
			return "", err
		}
		return result.ID.Hex(), nil
	}
	if conn.ID.IsZero() {
		conn.ID = primitive.NewObjectID()
	}
	if _, err := r.connections.InsertOne(ctx, conn); err != nil {
		return "", err
	}
	return conn.ID.Hex(), nil
}

func (r *MongoDriveRepository) FindConnection(ctx context.Context, id string) (models.DriveConnection, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.DriveConnection{}, mongo.ErrNoDocuments
	}
	var conn models.DriveConnection
	if err := r.connections.FindOne(ctx, bson.M{"_id": oid}).Decode(&conn); err != nil {
		return models.DriveConnection{}, err
	}
	return conn, nil
}

func (r *MongoDriveRepository) FindOperatorConnection(ctx context.Context) (models.DriveConnection, error) {
	var conn models.DriveConnection
	if err := r.connections.FindOne(ctx, bson.M{"kind": models.DriveConnectionOperator, "status": models.DriveConnectionActive}).Decode(&conn); err != nil {
		return models.DriveConnection{}, err
	}
	return conn, nil
}

func (r *MongoDriveRepository) FindCustomerConnection(ctx context.Context, customerID string) (models.DriveConnection, error) {
	var conn models.DriveConnection
	if err := r.connections.FindOne(ctx, bson.M{
		"kind": models.DriveConnectionCustomer, "customerId": customerID, "status": models.DriveConnectionActive,
	}).Decode(&conn); err != nil {
		return models.DriveConnection{}, err
	}
	return conn, nil
}

func (r *MongoDriveRepository) CreateOAuthState(ctx context.Context, state models.DriveOAuthState) error {
	_, err := r.oauthStates.InsertOne(ctx, state)
	return err
}

func (r *MongoDriveRepository) ConsumeOAuthState(ctx context.Context, stateHash string, now time.Time) (models.DriveOAuthState, error) {
	var state models.DriveOAuthState
	err := r.oauthStates.FindOneAndDelete(ctx, bson.M{"stateHash": stateHash, "expiresAt": bson.M{"$gt": now}}).Decode(&state)
	return state, err
}

func (r *MongoDriveRepository) CreateSetupToken(ctx context.Context, token models.DriveSetupToken) error {
	_, err := r.setupTokens.InsertOne(ctx, token)
	return err
}

func (r *MongoDriveRepository) FindSetupToken(ctx context.Context, tokenHash string) (models.DriveSetupToken, error) {
	var token models.DriveSetupToken
	if err := r.setupTokens.FindOne(ctx, bson.M{"tokenHash": tokenHash}).Decode(&token); err != nil {
		return models.DriveSetupToken{}, err
	}
	return token, nil
}

func (r *MongoDriveRepository) MarkSetupTokenUsed(ctx context.Context, tokenHash string, now time.Time) error {
	_, err := r.setupTokens.UpdateOne(ctx, bson.M{"tokenHash": tokenHash}, bson.M{"$set": bson.M{"usedAt": now}})
	return err
}

func (r *MongoDriveRepository) SetSetupConnection(ctx context.Context, tokenHash, connectionID string) error {
	_, err := r.setupTokens.UpdateOne(ctx, bson.M{"tokenHash": tokenHash}, bson.M{"$set": bson.M{"connectionId": connectionID}})
	return err
}

func (r *MongoDriveRepository) UpsertDestination(ctx context.Context, destination models.DriveDestination) error {
	now := time.Now()
	if destination.CreatedAt.IsZero() {
		destination.CreatedAt = now
	}
	destination.UpdatedAt = now
	_, err := r.destinations.UpdateOne(
		ctx,
		bson.M{"eventId": destination.EventID},
		bson.M{
			"$set": bson.M{
				"eventId":       destination.EventID,
				"connectionId":  destination.ConnectionID,
				"folderId":      destination.FolderID,
				"folderName":    destination.FolderName,
				"eventFolderId": destination.EventFolderID,
				"updatedAt":     now,
			},
			"$setOnInsert": bson.M{"createdAt": now},
		},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *MongoDriveRepository) FindDestinationByEvent(ctx context.Context, eventID string) (models.DriveDestination, error) {
	var destination models.DriveDestination
	if err := r.destinations.FindOne(ctx, bson.M{"eventId": eventID}).Decode(&destination); err != nil {
		return models.DriveDestination{}, err
	}
	return destination, nil
}

func (r *MongoDriveRepository) FindOrCreateGuestMapping(ctx context.Context, mapping models.DriveGuestMapping) (models.DriveGuestMapping, error) {
	filter := bson.M{"eventId": mapping.EventID, "guestIdHash": mapping.GuestIDHash}
	var existing models.DriveGuestMapping
	if err := r.guests.FindOne(ctx, filter).Decode(&existing); err == nil {
		return existing, nil
	}
	now := time.Now()
	mapping.CreatedAt = now
	mapping.UpdatedAt = now
	if _, err := r.guests.InsertOne(ctx, mapping); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			if findErr := r.guests.FindOne(ctx, filter).Decode(&existing); findErr == nil {
				return existing, nil
			}
		}
		return models.DriveGuestMapping{}, err
	}
	return mapping, nil
}

func (r *MongoDriveRepository) SetGuestFolder(ctx context.Context, eventID, guestIDHash, folderID string) error {
	_, err := r.guests.UpdateOne(ctx, bson.M{"eventId": eventID, "guestIdHash": guestIDHash}, bson.M{"$set": bson.M{"folderId": folderID, "updatedAt": time.Now()}})
	return err
}

func (r *MongoDriveRepository) InsertExportJobIfMissing(ctx context.Context, job models.DriveExportJob) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	if job.State == "" {
		job.State = models.DriveExportPending
	}
	_, err := r.jobs.InsertOne(ctx, job)
	if mongo.IsDuplicateKeyError(err) {
		return nil
	}
	return err
}

func (r *MongoDriveRepository) ClaimExportJob(ctx context.Context, now time.Time, lease time.Duration) (models.DriveExportJob, bool, error) {
	filter := bson.M{"$or": []bson.M{
		{"state": models.DriveExportPending},
		{"state": models.DriveExportExporting, "leaseUntil": bson.M{"$lt": now}},
	}}
	update := bson.M{
		"$set": bson.M{"state": models.DriveExportExporting, "leaseUntil": now.Add(lease), "updatedAt": now},
		"$inc": bson.M{"attempts": 1},
	}
	opts := options.FindOneAndUpdate().SetSort(bson.D{{Key: "createdAt", Value: 1}}).SetReturnDocument(options.After)
	var job models.DriveExportJob
	if err := r.jobs.FindOneAndUpdate(ctx, filter, update, opts).Decode(&job); err != nil {
		if err == mongo.ErrNoDocuments {
			return models.DriveExportJob{}, false, nil
		}
		return models.DriveExportJob{}, false, err
	}
	return job, true, nil
}

func (r *MongoDriveRepository) CompleteExportJob(ctx context.Context, id, mediaFileID, messageFileID string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.jobs.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"state": models.DriveExportExported, "mediaFileId": mediaFileID, "messageFileId": messageFileID,
		"errorCode": "", "leaseUntil": nil, "updatedAt": time.Now(),
	}})
	return err
}

func (r *MongoDriveRepository) FailExportJob(ctx context.Context, id, code string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.jobs.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"state": models.DriveExportFailed, "errorCode": code, "leaseUntil": nil, "updatedAt": time.Now(),
	}})
	return err
}

func (r *MongoDriveRepository) RescheduleExportJob(ctx context.Context, id, code string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.jobs.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{
		"state": models.DriveExportPending, "errorCode": code, "leaseUntil": nil, "updatedAt": time.Now(),
	}})
	return err
}

func (r *MongoDriveRepository) RetryFailedByEvent(ctx context.Context, eventID string) (int64, error) {
	result, err := r.jobs.UpdateMany(ctx, bson.M{"eventId": eventID, "state": models.DriveExportFailed}, bson.M{"$set": bson.M{
		"state": models.DriveExportPending, "errorCode": "", "leaseUntil": nil, "attempts": 0, "updatedAt": time.Now(),
	}})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *MongoDriveRepository) ResetExportsByEvent(ctx context.Context, eventID string) (int64, error) {
	result, err := r.jobs.UpdateMany(ctx, bson.M{"eventId": eventID}, bson.M{"$set": bson.M{
		"state": models.DriveExportPending, "errorCode": "", "leaseUntil": nil,
		"attempts": 0, "mediaFileId": "", "messageFileId": "", "updatedAt": time.Now(),
	}})
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

func (r *MongoDriveRepository) SetExportJobGuestHash(ctx context.Context, mediaID, guestHash string) error {
	_, err := r.jobs.UpdateMany(ctx, bson.M{"mediaId": mediaID}, bson.M{"$set": bson.M{"guestIdHash": guestHash, "updatedAt": time.Now()}})
	return err
}

func (r *MongoDriveRepository) SetDestinationEventFolder(ctx context.Context, eventID, folderID string) error {
	_, err := r.destinations.UpdateOne(ctx, bson.M{"eventId": eventID}, bson.M{"$set": bson.M{"eventFolderId": folderID, "updatedAt": time.Now()}})
	return err
}

func (r *MongoDriveRepository) DeleteByMediaID(ctx context.Context, mediaID string) error {
	_, err := r.jobs.DeleteMany(ctx, bson.M{"mediaId": mediaID})
	return err
}

func (r *MongoDriveRepository) ExportSummaryByEvent(ctx context.Context, eventID string) (map[string]int64, error) {
	cur, err := r.jobs.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"eventId": eventID}}},
		{{Key: "$group", Value: bson.M{"_id": "$state", "count": bson.M{"$sum": 1}}}},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := map[string]int64{"pending": 0, "exporting": 0, "exported": 0, "failed": 0}
	for cur.Next(ctx) {
		var row struct {
			State string `bson:"_id"`
			Count int64  `bson:"count"`
		}
		if err := cur.Decode(&row); err != nil {
			return nil, err
		}
		out[row.State] = row.Count
	}
	return out, cur.Err()
}
