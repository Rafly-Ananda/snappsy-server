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

type MongoUploadIntentRepository struct {
	collection *mongo.Collection
}

func NewMongoUploadIntentRepository(col *mongo.Collection) *MongoUploadIntentRepository {
	return &MongoUploadIntentRepository{collection: col}
}

func (r *MongoUploadIntentRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetName("idx_upload_intents_ttl").SetExpireAfterSeconds(0),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create upload intent indexes: %w", err)
	}
	return nil
}

func (r *MongoUploadIntentRepository) Insert(ctx context.Context, intent models.UploadIntent) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.InsertOne(ctx, intent)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoUploadIntentRepository) FindByID(ctx context.Context, id string) (models.UploadIntent, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.UploadIntent{}, mongo.ErrNoDocuments
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var intent models.UploadIntent
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&intent); err != nil {
		return models.UploadIntent{}, err
	}
	return intent, nil
}

func (r *MongoUploadIntentRepository) Complete(ctx context.Context, id string, mediaID string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return mongo.ErrNoDocuments
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": bson.M{"mediaId": mediaID}})
	return err
}
