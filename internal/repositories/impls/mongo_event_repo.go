package impls

import (
	"context"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MongoEventRepository struct {
	collection *mongo.Collection
}

func NewMongoEventRepository(col *mongo.Collection) *MongoEventRepository {
	return &MongoEventRepository{collection: col}
}

func (r *MongoEventRepository) Insert(ctx context.Context, event models.Events) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.InsertOne(ctx, event)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}
