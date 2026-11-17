package impls

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoMediaJobRepository struct {
	collection *mongo.Collection
}

func NewMongoMediaJobRepository(col *mongo.Collection) *MongoMediaJobRepository {
	return &MongoMediaJobRepository{collection: col}
}

func (r *MongoMediaJobRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "state", Value: 1}, {Key: "createdAt", Value: 1}},
			Options: options.Index().SetName("idx_media_jobs_state_created"),
		},
		{
			Keys:    bson.D{{Key: "mediaId", Value: 1}},
			Options: options.Index().SetName("idx_media_jobs_media_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "eventId", Value: 1}, {Key: "state", Value: 1}},
			Options: options.Index().SetName("idx_media_jobs_event_state"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create media job indexes: %w", err)
	}
	return nil
}

func (r *MongoMediaJobRepository) Insert(ctx context.Context, job models.MediaJob) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.InsertOne(ctx, job)
	return err
}

func (r *MongoMediaJobRepository) ClaimNext(ctx context.Context) (models.MediaJob, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	lease := now.Add(10 * time.Minute)
	filter := bson.M{
		"$or": []bson.M{
			{"state": models.MediaJobQueued},
			{"state": models.MediaJobProcessing, "leaseUntil": bson.M{"$lt": now}},
		},
	}
	update := bson.M{
		"$set": bson.M{"state": models.MediaJobProcessing, "leaseUntil": lease, "updatedAt": now},
		"$inc": bson.M{"attempts": 1},
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetSort(bson.D{{Key: "createdAt", Value: 1}})
	var job models.MediaJob
	if err := r.collection.FindOneAndUpdate(ctx, filter, update, opts).Decode(&job); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.MediaJob{}, mongo.ErrNoDocuments
		}
		return models.MediaJob{}, err
	}
	return job, nil
}

func (r *MongoMediaJobRepository) CountActiveByEvent(ctx context.Context, eventID string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return r.collection.CountDocuments(ctx, bson.M{
		"eventId": eventID,
		"state":   bson.M{"$in": []models.MediaJobState{models.MediaJobQueued, models.MediaJobProcessing}},
	})
}

func (r *MongoMediaJobRepository) SummaryByEvent(ctx context.Context, eventID string) (int64, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	active, err := r.collection.CountDocuments(ctx, bson.M{
		"eventId": eventID,
		"state":   bson.M{"$in": []models.MediaJobState{models.MediaJobQueued, models.MediaJobProcessing}},
	})
	if err != nil {
		return 0, 0, err
	}
	failed, err := r.collection.CountDocuments(ctx, bson.M{"eventId": eventID, "state": models.MediaJobFailed})
	return active, failed, err
}

func (r *MongoMediaJobRepository) Complete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = r.collection.UpdateByID(ctx, oid, bson.M{"$set": bson.M{"state": models.MediaJobReady, "leaseUntil": nil, "errorCode": "", "updatedAt": time.Now()}})
	return err
}

func (r *MongoMediaJobRepository) DeleteByMediaID(ctx context.Context, mediaID string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.DeleteMany(ctx, bson.M{"mediaId": mediaID})
	return err
}

func (r *MongoMediaJobRepository) Fail(ctx context.Context, id string, code string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = r.collection.UpdateByID(ctx, oid, bson.M{"$set": bson.M{"state": models.MediaJobFailed, "leaseUntil": nil, "errorCode": code, "updatedAt": time.Now()}})
	return err
}
