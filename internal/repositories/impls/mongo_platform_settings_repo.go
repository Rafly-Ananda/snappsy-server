package impls

import (
	"context"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoPlatformSettingsRepository struct {
	collection *mongo.Collection
}

func NewMongoPlatformSettingsRepository(collection *mongo.Collection) *MongoPlatformSettingsRepository {
	return &MongoPlatformSettingsRepository{collection: collection}
}

func (r *MongoPlatformSettingsRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := r.collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "key", Value: 1}},
		Options: options.Index().SetName("idx_platform_settings_key").SetUnique(true),
	})
	return err
}

func (r *MongoPlatformSettingsRepository) Find(ctx context.Context, key string) (models.PlatformSettings, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var settings models.PlatformSettings
	err := r.collection.FindOne(ctx, bson.M{"key": key}).Decode(&settings)
	if err == mongo.ErrNoDocuments {
		return models.PlatformSettings{}, false, nil
	}
	if err != nil {
		return models.PlatformSettings{}, false, err
	}
	return settings, true, nil
}

func (r *MongoPlatformSettingsRepository) Upsert(ctx context.Context, settings models.PlatformSettings) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	settings.UpdatedAt = time.Now()
	_, err := r.collection.UpdateOne(ctx,
		bson.M{"key": settings.Key},
		bson.M{"$set": bson.M{"demo": settings.Demo, "updatedAt": settings.UpdatedAt}, "$setOnInsert": bson.M{"key": settings.Key}},
		options.Update().SetUpsert(true),
	)
	return err
}
