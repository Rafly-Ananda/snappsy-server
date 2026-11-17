package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/config"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func RunMongo(ctx context.Context, db *mongo.Database, cfg config.MongoConfig) error {
	if err := migrateEvents(ctx, db.Collection(cfg.EventCollection)); err != nil {
		return fmt.Errorf("migrate events: %w", err)
	}
	if cfg.LegacyImageCollection != cfg.ImageCollection {
		if err := migrateMediaItems(ctx, db.Collection(cfg.ImageCollection), db.Collection(cfg.LegacyImageCollection)); err != nil {
			return fmt.Errorf("migrate media items: %w", err)
		}
	}
	return nil
}

func migrateEvents(ctx context.Context, collection *mongo.Collection) error {
	findCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cur, err := collection.Find(findCtx, bson.M{})
	if err != nil {
		return err
	}
	defer cur.Close(findCtx)

	type eventDoc struct {
		raw  bson.M
		id   primitive.ObjectID
		slug string
	}
	var docs []eventDoc
	usedSlugs := make(map[string]bool)
	for cur.Next(findCtx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return err
		}
		id, ok := raw["_id"].(primitive.ObjectID)
		if !ok {
			continue
		}
		slug := stringValue(raw["slug"])
		if slug != "" {
			usedSlugs[slug] = true
		}
		docs = append(docs, eventDoc{raw: raw, id: id, slug: slug})
	}
	if err := cur.Err(); err != nil {
		return err
	}

	now := time.Now()
	for _, doc := range docs {
		if doc.raw["countdowns"] == nil {
			set := bson.M{"countdowns": bson.M{"photo": 3, "strip": 3, "boomerang": 3}}
			if _, err := collection.UpdateOne(ctx, bson.M{"_id": doc.id}, bson.M{"$set": set}); err != nil {
				return err
			}
		}
		set := bson.M{}
		name := stringValue(doc.raw["name"])
		if name == "" {
			name = stringValue(doc.raw["eventName"])
			if name == "" {
				name = "Untitled Event"
			}
			set["name"] = name
		}
		if _, ok := doc.raw["description"]; !ok {
			set["description"] = ""
		}
		if doc.slug == "" {
			suffix := doc.id.Hex()
			if len(suffix) > 6 {
				suffix = suffix[len(suffix)-6:]
			}
			slug := identity.UniqueSlug(name, suffix, usedSlugs)
			usedSlugs[slug] = true
			set["slug"] = slug
		}
		if status := stringValue(doc.raw["status"]); status == "" {
			// Existing wedding events stay publicly reachable until the operator archives them.
			set["status"] = models.EventStatusLive
		}
		if _, ok := doc.raw["uploadPolicy"]; !ok {
			set["uploadPolicy"] = models.EventUploadPolicy{AcceptUploads: true}
		}
		if numberValue(doc.raw["retentionDays"]) == 0 {
			set["retentionDays"] = 30
		}
		if stringValue(doc.raw["ownerType"]) == "" {
			set["ownerType"] = models.EventOwnerPlatform
		}
		if stringValue(doc.raw["billingState"]) == "" {
			set["billingState"] = models.EventBillingActive
		}
		if _, ok := doc.raw["captureModes"]; !ok {
			set["captureModes"] = models.EventCaptureModes{Photo: true, Strip: true, Boomerang: true}
		}
		if _, ok := doc.raw["planSnapshot"]; !ok {
			set["planCode"] = "platform"
			set["planSnapshot"] = models.EventPlanSnapshot{
				Code: "platform", Name: "Platform", Currency: "IDR",
				PhotoQuota: -1, BoomerangQuota: -1, RetentionDays: 30,
			}
		}
		if stringValue(doc.raw["displayTokenHash"]) == "" {
			_, hash, err := identity.NewDisplayToken()
			if err != nil {
				return err
			}
			set["displayTokenHash"] = hash
		}
		createdAt, ok := timeValue(doc.raw["createdAt"])
		if !ok {
			createdAt = now
			set["createdAt"] = createdAt
		}
		if _, ok := doc.raw["updatedAt"]; !ok {
			set["updatedAt"] = createdAt
		}
		if len(set) == 0 {
			continue
		}
		updateCtx, updateCancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := collection.UpdateByID(updateCtx, doc.id, bson.M{"$set": set})
		updateCancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func migrateMediaItems(ctx context.Context, target *mongo.Collection, source *mongo.Collection) error {
	findCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cur, err := source.Find(findCtx, bson.M{})
	if err != nil {
		return err
	}
	defer cur.Close(findCtx)

	now := time.Now()
	for cur.Next(findCtx) {
		var raw bson.M
		if err := cur.Decode(&raw); err != nil {
			return err
		}
		id, ok := raw["_id"].(primitive.ObjectID)
		if !ok {
			continue
		}
		if stringValue(raw["kind"]) == "" {
			raw["kind"] = models.MediaKindPhoto
		}
		if stringValue(raw["objectKey"]) == "" {
			if key := stringValue(raw["minioKey"]); key != "" {
				raw["objectKey"] = key
			}
		}
		delete(raw, "minioKey")
		if _, ok := raw["createdAt"]; !ok {
			raw["createdAt"] = now
		}
		if _, ok := raw["updatedAt"]; !ok {
			raw["updatedAt"] = raw["createdAt"]
		}
		replaceCtx, replaceCancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := target.ReplaceOne(replaceCtx, bson.M{"_id": id}, raw, options.Replace().SetUpsert(true))
		replaceCancel()
		if err != nil {
			return err
		}
	}
	return cur.Err()
}

func stringValue(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func numberValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func timeValue(value any) (time.Time, bool) {
	switch v := value.(type) {
	case time.Time:
		return v, true
	case primitive.DateTime:
		return v.Time(), true
	default:
		return time.Time{}, false
	}
}
