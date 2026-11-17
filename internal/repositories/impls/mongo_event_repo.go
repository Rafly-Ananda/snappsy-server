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

type MongoEventRepository struct {
	collection *mongo.Collection
}

func NewMongoEventRepository(col *mongo.Collection) *MongoEventRepository {
	return &MongoEventRepository{collection: col}
}

func (r *MongoEventRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetName("idx_events_slug_unique").SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "displayTokenHash", Value: 1}},
			Options: options.Index().SetName("idx_events_display_token_unique").SetUnique(true).SetSparse(true),
		},
		{
			Keys:    bson.D{{Key: "status", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_events_status_created_at"),
		},
		{
			Keys:    bson.D{{Key: "ownerType", Value: 1}, {Key: "ownerId", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_events_owner_created_at"),
		},
		{
			Keys:    bson.D{{Key: "billingState", Value: 1}, {Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_events_billing_created_at"),
		},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		return fmt.Errorf("create event indexes: %w", err)
	}
	return nil
}

func (r *MongoEventRepository) Insert(ctx context.Context, event models.Event) (string, error) {
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

func (r *MongoEventRepository) Update(ctx context.Context, event models.Event) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	update := bson.M{"$set": event}
	unset := bson.M{}
	if event.TemplateID == "" {
		unset["templateId"] = 1
		unset["templateVersion"] = 1
	}
	if event.StripTemplateID == "" {
		unset["stripTemplateId"] = 1
		unset["stripTemplateVersion"] = 1
	}
	if event.BoomerangTemplateID == "" {
		unset["boomerangTemplateId"] = 1
		unset["boomerangTemplateVersion"] = 1
	}
	if len(event.AllowedTemplateIDs) == 0 {
		unset["allowedTemplateIds"] = 1
	}
	if len(event.HiddenStickerIDs) == 0 {
		unset["hiddenStickerIds"] = 1
	}
	if len(unset) > 0 {
		update["$unset"] = unset
	}
	res, err := r.collection.UpdateByID(ctx, event.ID, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *MongoEventRepository) FindByID(ctx context.Context, id string) (models.Event, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.Event{}, mongo.ErrNoDocuments
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

func (r *MongoEventRepository) FindBySlug(ctx context.Context, slug string) (models.Event, error) {
	return r.findOne(ctx, bson.M{"slug": slug})
}

func (r *MongoEventRepository) FindByDisplayTokenHash(ctx context.Context, hash string) (models.Event, error) {
	return r.findOne(ctx, bson.M{"displayTokenHash": hash})
}

func (r *MongoEventRepository) List(ctx context.Context) ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []models.Event
	for cur.Next(ctx) {
		var event models.Event
		if err := cur.Decode(&event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *MongoEventRepository) ListByOwner(ctx context.Context, ownerType models.EventOwnerType, ownerID string) ([]models.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	filter := bson.M{"ownerType": ownerType}
	if ownerID != "" {
		filter["ownerId"] = ownerID
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var events []models.Event
	for cur.Next(ctx) {
		var event models.Event
		if err := cur.Decode(&event); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, cur.Err()
}

func (r *MongoEventRepository) ConsumeQuota(ctx context.Context, id string, kind models.MediaKind, allowOverQuota bool) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, mongo.ErrNoDocuments
	}
	field, quotaField := "usedPhotos", "planSnapshot.photoQuota"
	if kind == models.MediaKindBoomerang {
		field, quotaField = "usedBoomerangs", "planSnapshot.boomerangQuota"
	}

	filter := bson.M{"_id": oid}
	if !allowOverQuota {
		filter["$or"] = bson.A{
			bson.M{quotaField: -1},
			bson.M{"$expr": bson.M{"$lt": bson.A{"$" + field, "$" + quotaField}}},
		}
	}
	res, err := r.collection.UpdateOne(ctx, filter, bson.M{"$inc": bson.M{field: 1}})
	if err != nil {
		return false, err
	}
	if res.MatchedCount == 0 {
		return false, nil
	}
	return true, nil
}

func (r *MongoEventRepository) UpdateTemplateVersion(ctx context.Context, templateID string, version int) error {
	fields := []string{"templateVersion", "stripTemplateVersion", "boomerangTemplateVersion"}
	ids := []string{"templateId", "stripTemplateId", "boomerangTemplateId"}
	for index, field := range fields {
		if _, err := r.collection.UpdateMany(ctx, bson.M{ids[index]: templateID}, bson.M{"$set": bson.M{field: version}}); err != nil {
			return err
		}
	}
	return nil
}

func (r *MongoEventRepository) ClearTemplateReference(ctx context.Context, templateID string) error {
	fields := []string{"templateId", "stripTemplateId", "boomerangTemplateId"}
	for _, field := range fields {
		unset := bson.M{field: ""}
		if field == "templateId" {
			unset["templateVersion"] = ""
		}
		if field == "stripTemplateId" {
			unset["stripTemplateVersion"] = ""
		}
		if field == "boomerangTemplateId" {
			unset["boomerangTemplateVersion"] = ""
		}
		if _, err := r.collection.UpdateMany(ctx, bson.M{field: templateID}, bson.M{"$unset": unset}); err != nil {
			return err
		}
	}
	return nil
}

func (r *MongoEventRepository) ReleaseQuota(ctx context.Context, id string, kind models.MediaKind) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return mongo.ErrNoDocuments
	}
	field := "usedPhotos"
	if kind == models.MediaKindBoomerang {
		field = "usedBoomerangs"
	}
	_, err = r.collection.UpdateOne(ctx, bson.M{"_id": oid, field: bson.M{"$gt": 0}}, bson.M{"$inc": bson.M{field: -1}})
	return err
}

func (r *MongoEventRepository) findOne(ctx context.Context, filter bson.M) (models.Event, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var event models.Event
	if err := r.collection.FindOne(ctx, filter).Decode(&event); err != nil {
		return models.Event{}, err
	}
	return event, nil
}
