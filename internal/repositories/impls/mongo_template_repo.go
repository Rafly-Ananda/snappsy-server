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

type MongoTemplateRepository struct {
	templates *mongo.Collection
	versions  *mongo.Collection
}

func NewMongoTemplateRepository(templates *mongo.Collection, versions *mongo.Collection) *MongoTemplateRepository {
	return &MongoTemplateRepository{templates: templates, versions: versions}
}

func (r *MongoTemplateRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	templateIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "createdAt", Value: -1}},
			Options: options.Index().SetName("idx_templates_created_at"),
		},
	}
	if _, err := r.templates.Indexes().CreateMany(ctx, templateIndexes); err != nil {
		return fmt.Errorf("create template indexes: %w", err)
	}

	versionIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "templateId", Value: 1}, {Key: "version", Value: 1}},
			Options: options.Index().SetName("idx_template_versions_unique").SetUnique(true),
		},
	}
	if _, err := r.versions.Indexes().CreateMany(ctx, versionIndexes); err != nil {
		return fmt.Errorf("create template version indexes: %w", err)
	}
	return nil
}

func (r *MongoTemplateRepository) InsertTemplate(ctx context.Context, template models.Template) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.templates.InsertOne(ctx, template)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoTemplateRepository) UpdateTemplate(ctx context.Context, template models.Template) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.templates.UpdateByID(ctx, template.ID, bson.M{"$set": template})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *MongoTemplateRepository) FindTemplateByID(ctx context.Context, id string) (models.Template, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return models.Template{}, mongo.ErrNoDocuments
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var template models.Template
	if err := r.templates.FindOne(ctx, bson.M{"_id": oid}).Decode(&template); err != nil {
		return models.Template{}, err
	}
	return template, nil
}

func (r *MongoTemplateRepository) ListTemplates(ctx context.Context) ([]models.Template, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.templates.Find(ctx, bson.M{"eventId": bson.M{"$in": bson.A{"", nil}}}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	templates := make([]models.Template, 0)
	for cur.Next(ctx) {
		var template models.Template
		if err := cur.Decode(&template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return templates, nil
}

func (r *MongoTemplateRepository) ListTemplatesByEvent(ctx context.Context, eventID string, kind string) ([]models.Template, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	filter := bson.M{"eventId": eventID}
	if kind != "" {
		filter["kind"] = kind
	}
	opts := options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}})
	cur, err := r.templates.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	templates := make([]models.Template, 0)
	for cur.Next(ctx) {
		var template models.Template
		if err := cur.Decode(&template); err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, cur.Err()
}

func (r *MongoTemplateRepository) InsertVersion(ctx context.Context, version models.TemplateVersion) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.versions.InsertOne(ctx, version)
	if err != nil {
		return "", err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		return oid.Hex(), nil
	}
	return "", nil
}

func (r *MongoTemplateRepository) UpdateVersion(ctx context.Context, version models.TemplateVersion) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	res, err := r.versions.UpdateByID(ctx, version.ID, bson.M{"$set": version})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *MongoTemplateRepository) FindVersion(ctx context.Context, templateID string, version int) (models.TemplateVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var item models.TemplateVersion
	if err := r.versions.FindOne(ctx, bson.M{"templateId": templateID, "version": version}).Decode(&item); err != nil {
		return models.TemplateVersion{}, err
	}
	return item, nil
}

func (r *MongoTemplateRepository) DeleteTemplate(ctx context.Context, templateID string) error {
	oid, err := primitive.ObjectIDFromHex(templateID)
	if err != nil {
		return err
	}
	_, err = r.templates.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}

func (r *MongoTemplateRepository) DeleteVersions(ctx context.Context, templateID string) error {
	_, err := r.versions.DeleteMany(ctx, bson.M{"templateId": templateID})
	return err
}

func (r *MongoTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]models.TemplateVersion, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(bson.D{{Key: "version", Value: -1}})
	cur, err := r.versions.Find(ctx, bson.M{"templateId": templateID}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	versions := make([]models.TemplateVersion, 0)
	for cur.Next(ctx) {
		var item models.TemplateVersion
		if err := cur.Decode(&item); err != nil {
			return nil, err
		}
		versions = append(versions, item)
	}
	if err := cur.Err(); err != nil {
		return nil, err
	}
	return versions, nil
}
