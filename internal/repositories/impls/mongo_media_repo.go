package impls

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoMediaRepository struct {
	collection *mongo.Collection
}

func NewMongoMediaRepository(col *mongo.Collection) *MongoMediaRepository {
	return &MongoMediaRepository{collection: col}
}

func (r *MongoMediaRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "eventId", Value: 1},
				{Key: "createdAt", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName("idx_media_items_event_created_at_id"),
		},
		{
			Keys: bson.D{
				{Key: "eventId", Value: 1},
				{Key: "moderationState", Value: 1},
				{Key: "createdAt", Value: -1},
				{Key: "_id", Value: -1},
			},
			Options: options.Index().SetName("idx_media_items_event_state_created_at_id"),
		},
		{
			Keys:    bson.D{{Key: "objectKey", Value: 1}},
			Options: options.Index().SetName("idx_media_items_object_key_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "uploadIntentId", Value: 1}},
			Options: options.Index().SetName("idx_media_items_upload_intent_unique").SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "retentionAt", Value: 1}},
			Options: options.Index().SetName("idx_media_items_retention").SetSparse(true),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create media item indexes: %w", err)
	}
	return nil
}

func (r *MongoMediaRepository) Insert(ctx context.Context, item models.MediaItem) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.InsertOne(ctx, item)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoMediaRepository) Update(ctx context.Context, item models.MediaItem) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.UpdateByID(ctx, item.ID, bson.M{"$set": item})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *MongoMediaRepository) FindByID(ctx context.Context, id string) (models.MediaItem, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.MediaItem{}, mongo.ErrNoDocuments
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var item models.MediaItem
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&item); err != nil {
		return models.MediaItem{}, err
	}
	return item, nil
}

func (r *MongoMediaRepository) FindByUploadIntentID(ctx context.Context, intentID string) (models.MediaItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var item models.MediaItem
	if err := r.collection.FindOne(ctx, bson.M{"uploadIntentId": intentID}).Decode(&item); err != nil {
		return models.MediaItem{}, err
	}
	return item, nil
}

func (r *MongoMediaRepository) FindExpired(ctx context.Context, now time.Time, limit int) ([]models.MediaItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cur, err := r.collection.Find(ctx, bson.M{"retentionAt": bson.M{"$lte": now}}, options.Find().
		SetSort(bson.D{{Key: "retentionAt", Value: 1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []models.MediaItem
	for cur.Next(ctx) {
		var item models.MediaItem
		if err := cur.Decode(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, cur.Err()
}

func (r *MongoMediaRepository) Delete(ctx context.Context, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = r.collection.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (r *MongoMediaRepository) FindReadyByEvent(ctx context.Context, eventId string) ([]models.MediaItem, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cur, err := r.collection.Find(ctx, bson.M{"eventId": eventId, "processingState": models.MediaProcessingReady}, options.Find().SetLimit(1000))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	var out []models.MediaItem
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *MongoMediaRepository) FindAllByEvents(ctx context.Context, eventId string, moderationState string, after string, limit int) ([]models.MediaItem, string, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"eventId": eventId}
	if moderationState != "" {
		filter["moderationState"] = moderationState
	}
	if after != "" {
		ca, oid, ok := parseCursor(after)
		if !ok {
			return nil, "", fmt.Errorf("invalid cursor")
		}
		filter = bson.M{
			"eventId": eventId,
			"$or": []bson.M{
				{"createdAt": bson.M{"$lt": ca}},
				{
					"createdAt": ca,
					"_id":       bson.M{"$lt": oid},
				},
			},
		}
		if moderationState != "" {
			filter["moderationState"] = moderationState
		}
	}

	opts := options.Find().
		SetSort(bson.D{
			{Key: "createdAt", Value: -1},
			{Key: "_id", Value: -1},
		}).
		SetLimit(int64(limit))

	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, "", err
	}
	defer cur.Close(ctx)

	var out []models.MediaItem
	for cur.Next(ctx) {
		var item models.MediaItem
		if err := cur.Decode(&item); err != nil {
			return nil, "", err
		}
		out = append(out, item)
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}

	next := ""
	if len(out) == limit {
		last := out[len(out)-1]
		next = makeCursor(last.CreatedAt, last.ID)
	}

	return out, next, nil
}

func makeCursor(t time.Time, id primitive.ObjectID) string {
	raw := t.UTC().Format(time.RFC3339Nano) + "|" + id.Hex()
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

func parseCursor(s string) (time.Time, primitive.ObjectID, bool) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, primitive.NilObjectID, false
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, primitive.NilObjectID, false
	}

	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, primitive.NilObjectID, false
	}

	oid, err := primitive.ObjectIDFromHex(parts[1])
	if err != nil {
		return time.Time{}, primitive.NilObjectID, false
	}

	return t, oid, true
}
