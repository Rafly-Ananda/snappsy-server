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

type MongoAssetRepository struct {
	collection *mongo.Collection
}

func NewMongoAssetRepository(col *mongo.Collection) *MongoAssetRepository {
	return &MongoAssetRepository{collection: col}
}

func (r *MongoAssetRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "kind", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_assets_kind_created_at"),
		},
		{
			Keys:    bson.D{{Key: "objectKey", Value: 1}},
			Options: options.Index().SetName("idx_assets_object_key_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "eventId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_assets_event_created_at"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create asset indexes: %w", err)
	}
	return nil
}

func (r *MongoAssetRepository) Insert(ctx context.Context, asset models.Asset) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.collection.InsertOne(ctx, asset)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoAssetRepository) List(ctx context.Context, kind string) ([]models.Asset, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"eventId": bson.M{"$in": bson.A{"", nil}}}
	if kind != "" {
		filter["kind"] = kind
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	assets := make([]models.Asset, 0)
	for cur.Next(ctx) {
		var asset models.Asset
		if err := cur.Decode(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return assets, nil
}

func (r *MongoAssetRepository) ListPlatform(ctx context.Context, kind string) ([]models.Asset, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	filter := bson.M{"eventId": bson.M{"$in": bson.A{"", nil}}}
	if kind != "" {
		filter["kind"] = kind
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	assets := make([]models.Asset, 0)
	for cur.Next(ctx) {
		var asset models.Asset
		if err := cur.Decode(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, cur.Err()
}

func (r *MongoAssetRepository) ListByEvent(ctx context.Context, eventID string, kind string) ([]models.Asset, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	filter := bson.M{"eventId": eventID}
	if kind != "" {
		filter["kind"] = kind
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	assets := make([]models.Asset, 0)
	for cur.Next(ctx) {
		var asset models.Asset
		if err := cur.Decode(&asset); err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, cur.Err()
}

func (r *MongoAssetRepository) Update(ctx context.Context, asset models.Asset) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.UpdateOne(ctx, bson.M{"_id": asset.ID}, bson.M{"$set": bson.M{
		"name": asset.Name, "disabled": asset.Disabled, "updatedAt": time.Now(),
	}})
	return err
}

func (r *MongoAssetRepository) FindByID(ctx context.Context, id string) (models.Asset, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.Asset{}, mongo.ErrNoDocuments
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var asset models.Asset
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&asset); err != nil {
		return models.Asset{}, err
	}
	return asset, nil
}
