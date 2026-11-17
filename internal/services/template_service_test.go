package services

import (
	"context"
	"errors"
	"testing"
	"time"

	assetDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/assets"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeAssetRepository struct {
	assets map[string]models.Asset
}

func (f *fakeAssetRepository) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeAssetRepository) Insert(ctx context.Context, asset models.Asset) (string, error) {
	if f.assets == nil {
		f.assets = make(map[string]models.Asset)
	}
	f.assets[asset.ID.Hex()] = asset
	return asset.ID.Hex(), nil
}
func (f *fakeAssetRepository) List(ctx context.Context, kind string) ([]models.Asset, error) {
	out := make([]models.Asset, 0)
	for _, asset := range f.assets {
		if kind == "" || string(asset.Kind) == kind {
			out = append(out, asset)
		}
	}
	return out, nil
}
func (f *fakeAssetRepository) ListPlatform(ctx context.Context, kind string) ([]models.Asset, error) {
	return f.List(ctx, kind)
}
func (f *fakeAssetRepository) ListByEvent(ctx context.Context, eventID string, kind string) ([]models.Asset, error) {
	out := make([]models.Asset, 0)
	for _, asset := range f.assets {
		if asset.EventID == eventID && (kind == "" || string(asset.Kind) == kind) {
			out = append(out, asset)
		}
	}
	return out, nil
}
func (f *fakeAssetRepository) Update(ctx context.Context, asset models.Asset) error {
	f.assets[asset.ID.Hex()] = asset
	return nil
}
func (f *fakeAssetRepository) FindByID(ctx context.Context, id string) (models.Asset, error) {
	asset, ok := f.assets[id]
	if !ok {
		return models.Asset{}, mongo.ErrNoDocuments
	}
	return asset, nil
}

type fakeTemplateRepository struct {
	templates map[string]models.Template
	versions  map[string]map[int]models.TemplateVersion
}

func (f *fakeTemplateRepository) EnsureIndexes(ctx context.Context) error { return nil }
func (f *fakeTemplateRepository) InsertTemplate(ctx context.Context, template models.Template) (string, error) {
	if f.templates == nil {
		f.templates = make(map[string]models.Template)
	}
	f.templates[template.ID.Hex()] = template
	return template.ID.Hex(), nil
}
func (f *fakeTemplateRepository) UpdateTemplate(ctx context.Context, template models.Template) error {
	if _, ok := f.templates[template.ID.Hex()]; !ok {
		return mongo.ErrNoDocuments
	}
	f.templates[template.ID.Hex()] = template
	return nil
}
func (f *fakeTemplateRepository) FindTemplateByID(ctx context.Context, id string) (models.Template, error) {
	template, ok := f.templates[id]
	if !ok {
		return models.Template{}, mongo.ErrNoDocuments
	}
	return template, nil
}
func (f *fakeTemplateRepository) ListTemplates(ctx context.Context) ([]models.Template, error) {
	out := make([]models.Template, 0, len(f.templates))
	for _, template := range f.templates {
		out = append(out, template)
	}
	return out, nil
}
func (f *fakeTemplateRepository) ListTemplatesByEvent(ctx context.Context, eventID string, kind string) ([]models.Template, error) {
	out := make([]models.Template, 0)
	for _, template := range f.templates {
		if template.EventID == eventID && (kind == "" || string(template.Kind) == kind) {
			out = append(out, template)
		}
	}
	return out, nil
}
func (f *fakeTemplateRepository) InsertVersion(ctx context.Context, version models.TemplateVersion) (string, error) {
	if f.versions == nil {
		f.versions = make(map[string]map[int]models.TemplateVersion)
	}
	if f.versions[version.TemplateID] == nil {
		f.versions[version.TemplateID] = make(map[int]models.TemplateVersion)
	}
	f.versions[version.TemplateID][version.Version] = version
	return version.ID.Hex(), nil
}
func (f *fakeTemplateRepository) UpdateVersion(ctx context.Context, version models.TemplateVersion) error {
	if _, ok := f.versions[version.TemplateID][version.Version]; !ok {
		return mongo.ErrNoDocuments
	}
	f.versions[version.TemplateID][version.Version] = version
	return nil
}
func (f *fakeTemplateRepository) FindVersion(ctx context.Context, templateID string, version int) (models.TemplateVersion, error) {
	item, ok := f.versions[templateID][version]
	if !ok {
		return models.TemplateVersion{}, mongo.ErrNoDocuments
	}
	return item, nil
}
func (f *fakeTemplateRepository) DeleteTemplate(ctx context.Context, templateID string) error {
	return nil
}
func (f *fakeTemplateRepository) DeleteVersions(ctx context.Context, templateID string) error {
	return nil
}

func (f *fakeTemplateRepository) ListVersions(ctx context.Context, templateID string) ([]models.TemplateVersion, error) {
	out := make([]models.TemplateVersion, 0)
	for _, version := range f.versions[templateID] {
		out = append(out, version)
	}
	return out, nil
}

func TestTemplateDraftSaveAndReproduce(t *testing.T) {
	svc := NewTemplateService(&fakeTemplateRepository{}, &fakeAssetRepository{})
	created, err := svc.CreateTemplate(context.Background(), "Wedding Frame", "photo")
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	schema := created.Draft.Schema
	schema.Canvas.Background = "#14342A"
	schema.Layers = []models.TemplateLayer{{
		ID: "text-1", Type: models.TemplateLayerText, Text: "Rafly & Intan",
		X: 100, Y: 80, Width: 500, Height: 80,
	}}

	if _, err := svc.SaveDraft(context.Background(), created.ID, schema); err != nil {
		t.Fatalf("SaveDraft() error = %v", err)
	}
	loaded, err := svc.GetTemplate(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetTemplate() error = %v", err)
	}
	if loaded.Draft.Schema.Canvas.Background != "#14342A" || len(loaded.Draft.Schema.Layers) != 1 {
		t.Fatalf("draft did not reproduce after reload: %+v", loaded.Draft.Schema)
	}
}

func TestTemplatePublishCreatesNextDraftAndValidatesAssignment(t *testing.T) {
	svc := NewTemplateService(&fakeTemplateRepository{}, &fakeAssetRepository{})
	created, err := svc.CreateTemplate(context.Background(), "Wedding Frame", "photo")
	if err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	published, err := svc.Publish(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if published.LatestPublishedVer != 1 || published.CurrentDraftVersion != 2 {
		t.Fatalf("Publish() versions = published %d draft %d", published.LatestPublishedVer, published.CurrentDraftVersion)
	}
	if published.Draft.Schema.Canvas.Width != created.Draft.Schema.Canvas.Width {
		t.Fatal("next draft did not copy published schema")
	}
	if _, err := svc.ValidateAssignment(context.Background(), created.ID, 1); err != nil {
		t.Fatalf("ValidateAssignment(published) error = %v", err)
	}
	if _, err := svc.ValidateAssignment(context.Background(), created.ID, 2); !errors.Is(err, ErrTemplateAssignment) {
		t.Fatalf("ValidateAssignment(draft) error = %v", err)
	}
}

func TestTemplateSchemaValidationRejectsUnsafeLayers(t *testing.T) {
	assetID := primitive.NewObjectID().Hex()
	assets := &fakeAssetRepository{assets: map[string]models.Asset{
		assetID: {ID: mustObjectID(assetID), Kind: models.AssetKindSticker},
	}}
	svc := NewTemplateService(&fakeTemplateRepository{}, assets)
	schema := DefaultTemplateSchema()

	if err := svc.ValidateSchema(context.Background(), schema); err != nil {
		t.Fatalf("ValidateSchema(default) error = %v", err)
	}

	bad := schema
	bad.Layers = []models.TemplateLayer{{
		ID: "text-1", Type: models.TemplateLayerText,
		X: -10, Y: 0, Width: 100, Height: 50,
	}}
	if err := svc.ValidateSchema(context.Background(), bad); err == nil {
		t.Fatal("ValidateSchema() accepted an out-of-bounds layer")
	}

	bad = schema
	bad.Layers = []models.TemplateLayer{{
		ID: "image-1", Type: models.TemplateLayerImage, AssetID: primitive.NewObjectID().Hex(),
		X: 0, Y: 0, Width: 100, Height: 100,
	}}
	if err := svc.ValidateSchema(context.Background(), bad); err == nil {
		t.Fatal("ValidateSchema() accepted a missing image asset")
	}

	bad = schema
	bad.Layers = []models.TemplateLayer{{
		ID: "image-1", Type: models.TemplateLayerImage, AssetID: assetID,
		X: 0, Y: 0, Width: 100, Height: 100,
	}}
	if err := svc.ValidateSchema(context.Background(), bad); err != nil {
		t.Fatalf("ValidateSchema() rejected a valid image layer: %v", err)
	}
}

func TestAssetServiceUploadValidation(t *testing.T) {
	storage := &fakeObjectStorage{exists: true}
	repo := &fakeAssetRepository{}
	svc := NewAssetService(repo, storage, "images", time.Minute)

	presigned, err := svc.GeneratePresignedUpload(context.Background(), assetDto.PresignAssetUploadReq{
		Name:     "Floral sticker",
		Kind:     "sticker",
		MimeType: "image/png",
	})
	if err != nil {
		t.Fatalf("GeneratePresignedUpload() error = %v", err)
	}
	if presigned.ObjectKey == "" || len(storage.keys) != 1 || storage.keys[0] != presigned.ObjectKey {
		t.Fatalf("GeneratePresignedUpload() key = %q, storage keys = %v", presigned.ObjectKey, storage.keys)
	}

	created, err := svc.CreateAsset(context.Background(), assetDto.CreateAssetReq{
		Kind:      "sticker",
		Name:      "Floral sticker",
		ObjectKey: presigned.ObjectKey,
		MimeType:  "image/png",
	})
	if err != nil {
		t.Fatalf("CreateAsset() error = %v", err)
	}
	if created.ID == "" || created.Kind != "sticker" || created.Url == "" {
		t.Fatalf("CreateAsset() response = %+v", created)
	}
}

func mustObjectID(hex string) primitive.ObjectID {
	oid, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return oid
}
