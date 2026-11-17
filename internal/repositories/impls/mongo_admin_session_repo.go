package impls

import (
	"context"
	"fmt"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoAdminSessionRepository struct {
	collection *mongo.Collection
}

func NewMongoAdminSessionRepository(col *mongo.Collection) *MongoAdminSessionRepository {
	return &MongoAdminSessionRepository{collection: col}
}

func (r *MongoAdminSessionRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tokenHash", Value: 1}},
			Options: options.Index().SetName("idx_admin_sessions_token_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetName("idx_admin_sessions_expires_at"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create admin session indexes: %w", err)
	}
	return nil
}

func (r *MongoAdminSessionRepository) Insert(ctx context.Context, session models.AdminSession) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.InsertOne(ctx, session)
	return err
}

func (r *MongoAdminSessionRepository) FindByTokenHash(ctx context.Context, tokenHash string) (models.AdminSession, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var session models.AdminSession
	if err := r.collection.FindOne(ctx, bson.M{"tokenHash": tokenHash}).Decode(&session); err != nil {
		return models.AdminSession{}, err
	}
	return session, nil
}

func (r *MongoAdminSessionRepository) DeleteByTokenHash(ctx context.Context, tokenHash string) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.collection.DeleteOne(ctx, bson.M{"tokenHash": tokenHash})
	return err
}
