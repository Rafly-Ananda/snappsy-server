package impls

import (
	"context"
	"encoding/base64"
	"strings"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoImageRepository struct {
	collection *mongo.Collection
}

func NewMongoImageRepository(col *mongo.Collection) *MongoImageRepository {
	return &MongoImageRepository{collection: col}
}

func (r *MongoImageRepository) Insert(ctx context.Context, image models.Images) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.InsertOne(ctx, image)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoImageRepository) FindAllByEvents(ctx context.Context, eventId string, after string, limit int) ([]models.Images, string, error) {
	// TODO: need to put this into a constant
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{"eventId": eventId}
	// If we have a cursor, fetch items "older" than that point:
	// (createdAt < cursor.createdAt) OR
	// (createdAt == cursor.createdAt AND _id < cursor._id)
	if after != "" {
		ca, oid, ok := parseCursor(after)
		if ok {
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
		}
	}

	// Use the same order as the index for efficient scan.
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

	var out []models.Images
	for cur.Next(ctx) {
		var img models.Images
		if err := cur.Decode(&img); err != nil {
			return nil, "", err
		}
		out = append(out, img)
	}
	if err := cur.Err(); err != nil {
		return nil, "", err
	}

	// Build next cursor from the last doc
	next := ""
	if len(out) == limit {
		last := out[len(out)-1]

		next = makeCursor(last.CreatedAt, last.ID)
	}

	return out, next, nil
}

// TODO: might to move this Helpers for Cursor
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
