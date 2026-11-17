package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	templateDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/templates"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrTemplateNotFound        = errors.New("template not found")
	ErrTemplateVersionNotFound = errors.New("template version not found")
	ErrInvalidTemplateSchema   = errors.New("invalid template schema")
	ErrTemplateAssignment      = errors.New("template assignment must reference a published version")
	hexColorPattern            = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
)

const maxTemplateLayers = 20

type DesignValidator func(ctx context.Context, templateID string, version int) (int, error)

type TemplateService struct {
	repo   repositories.TemplateRepository
	assets repositories.AssetRepository
}

func NewTemplateService(repo repositories.TemplateRepository, assets repositories.AssetRepository) *TemplateService {
	return &TemplateService{repo: repo, assets: assets}
}

func (s *TemplateService) CreateTemplate(ctx context.Context, name, kind string) (templateDto.TemplateDetailRes, error) {
	now := time.Now()
	templateKind := models.TemplateKind(kind)
	if templateKind == "" {
		templateKind = models.TemplateKindPhoto
	}
	if templateKind != models.TemplateKindPhoto && templateKind != models.TemplateKindStrip && templateKind != models.TemplateKindVideoFrame {
		return templateDto.TemplateDetailRes{}, errors.New("template kind must be photo, strip or video_frame")
	}
	template := models.Template{
		ID:              primitive.NewObjectID(),
		Kind:            templateKind,
		Name:            strings.TrimSpace(name),
		CurrentDraftVer: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if template.Name == "" {
		return templateDto.TemplateDetailRes{}, errors.New("template name is required")
	}
	templateID, err := s.repo.InsertTemplate(ctx, template)
	if err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	template.ID, _ = primitive.ObjectIDFromHex(templateID)

	version := models.TemplateVersion{
		ID:         primitive.NewObjectID(),
		TemplateID: templateID,
		Version:    1,
		Status:     "draft",
		Schema:     DefaultTemplateSchema(),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := s.repo.InsertVersion(ctx, version); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	return s.GetTemplate(ctx, templateID)
}

func (s *TemplateService) CreateEventTemplate(ctx context.Context, ownerID, eventID string, req templateDto.CreateEventTemplateReq) (templateDto.TemplateDetailRes, error) {
	kind := models.TemplateKind(req.Kind)
	if kind != models.TemplateKindPhoto && kind != models.TemplateKindStrip && kind != models.TemplateKindVideoFrame {
		return templateDto.TemplateDetailRes{}, errors.New("template kind must be photo, strip or video_frame")
	}
	style := models.TemplateStyle{
		Preset: strings.TrimSpace(req.Preset),
		Paper:  defaultColor(req.Paper, "#F2E8D5"),
		Ink:    defaultColor(req.Ink, "#191714"),
		Accent: defaultColor(req.Accent, "#ED7648"),
		Title:  strings.TrimSpace(req.Title),
	}
	now := time.Now()
	template := models.Template{
		ID:              primitive.NewObjectID(),
		Kind:            kind,
		OwnerType:       string(models.EventOwnerCustomer),
		OwnerID:         ownerID,
		EventID:         eventID,
		Name:            strings.TrimSpace(req.Name),
		CurrentDraftVer: 1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if template.Name == "" {
		return templateDto.TemplateDetailRes{}, errors.New("template name is required")
	}
	templateID, err := s.repo.InsertTemplate(ctx, template)
	if err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	template.ID, _ = primitive.ObjectIDFromHex(templateID)
	schema := DefaultTemplateSchema()
	schema.Style = style
	if kind == models.TemplateKindPhoto {
		schema.Canvas.Background = style.Paper
	} else if kind == models.TemplateKindStrip {
		schema.Canvas.Width = 600
		schema.Canvas.Height = 1800
		schema.Canvas.Background = style.Paper
		schema.PhotoSlot = models.TemplatePhotoSlot{}
		schema.PhotoSlots = []models.TemplatePhotoSlot{
			{X: 40, Y: 100, Width: 520, Height: 350},
			{X: 40, Y: 466, Width: 520, Height: 350},
			{X: 40, Y: 832, Width: 520, Height: 350},
			{X: 40, Y: 1198, Width: 520, Height: 350},
		}
		schema.Layers = nil
	} else {
		schema.Canvas.Width = 720
		schema.Canvas.Height = 1280
		schema.Canvas.Background = style.Paper
		schema.PhotoSlot = models.TemplatePhotoSlot{X: 40, Y: 40, Width: 640, Height: 1040}
		schema.Layers = nil
	}
	if kind != models.TemplateKindPhoto {
		if err := s.validateStyle(style); err != nil {
			return templateDto.TemplateDetailRes{}, err
		}
	} else if err := s.ValidateSchema(ctx, schema); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	version := models.TemplateVersion{
		ID:         primitive.NewObjectID(),
		TemplateID: templateID,
		Version:    1,
		Status:     "published",
		Schema:     schema,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := s.repo.InsertVersion(ctx, version); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	template.LatestPublishedVer = 1
	template.CurrentDraftVer = 1
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	return s.GetTemplate(ctx, templateID)
}

func (s *TemplateService) UpdateEventTemplateStyle(ctx context.Context, customerID, eventID, templateID string, req templateDto.UpdateEventTemplateStyleReq) (templateDto.TemplateRes, error) {
	if err := s.ValidateEventAssignment(ctx, eventID, templateID, models.TemplateKind("")); err != nil {
		return templateDto.TemplateRes{}, err
	}
	template, err := s.repo.FindTemplateByID(ctx, templateID)
	if err != nil || template.EventID != eventID {
		return templateDto.TemplateRes{}, ErrTemplateNotFound
	}
	if len(strings.TrimSpace(req.Name)) == 0 || len(strings.TrimSpace(req.Name)) > 80 {
		return templateDto.TemplateRes{}, ErrInvalidTemplateSchema
	}
	if err := s.validateStyle(req.Style); err != nil {
		return templateDto.TemplateRes{}, err
	}

	template.Name = strings.TrimSpace(req.Name)
	template.UpdatedAt = time.Now()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return templateDto.TemplateRes{}, err
	}
	for _, versionNo := range uniqueInts([]int{template.CurrentDraftVer, template.LatestPublishedVer}) {
		if versionNo <= 0 {
			continue
		}
		version, err := s.repo.FindVersion(ctx, template.ID.Hex(), versionNo)
		if err != nil {
			continue
		}
		version.Schema.Style = req.Style
		version.UpdatedAt = time.Now()
		if err := s.repo.UpdateVersion(ctx, version); err != nil {
			return templateDto.TemplateRes{}, err
		}
	}
	return templateDto.NewTemplateRes(template), nil
}

func uniqueInts(values []int) []int {
	seen := make(map[int]bool, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func (s *TemplateService) ValidateEventAssignment(ctx context.Context, eventID, templateID string, kind models.TemplateKind) error {
	if strings.TrimSpace(templateID) == "" {
		return nil
	}
	template, err := s.repo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return ErrTemplateNotFound
	}
	if template.EventID != "" && template.EventID != eventID {
		return ErrTemplateNotFound
	}
	if kind != "" && template.Kind != "" && template.Kind != kind {
		return ErrTemplateAssignment
	}
	return nil
}

func (s *TemplateService) DeleteTemplate(ctx context.Context, templateID string) error {
	return s.delete(ctx, templateID)
}

func (s *TemplateService) DeleteEventTemplate(ctx context.Context, ownerID, eventID, templateID string) error {
	template, err := s.repo.FindTemplateByID(ctx, templateID)
	if err != nil || template.EventID != eventID || template.OwnerID != ownerID {
		return ErrTemplateNotFound
	}
	return s.delete(ctx, templateID)
}

func (s *TemplateService) delete(ctx context.Context, templateID string) error {
	if err := s.repo.DeleteVersions(ctx, templateID); err != nil {
		return err
	}
	return s.repo.DeleteTemplate(ctx, templateID)
}

func (s *TemplateService) PublishedSchema(ctx context.Context, templateID string) (models.TemplateSchema, error) {
	template, err := s.repo.FindTemplateByID(ctx, templateID)
	if err != nil || template.LatestPublishedVer == 0 {
		return models.TemplateSchema{}, ErrTemplateNotFound
	}
	version, err := s.repo.FindVersion(ctx, templateID, template.LatestPublishedVer)
	if err != nil || version.Status != "published" {
		return models.TemplateSchema{}, ErrTemplateVersionNotFound
	}
	return version.Schema, nil
}

func (s *TemplateService) ListPlatformTemplates(ctx context.Context, ids []string) ([]templateDto.TemplateRes, error) {
	out := make([]templateDto.TemplateRes, 0, len(ids))
	for _, id := range ids {
		template, err := s.repo.FindTemplateByID(ctx, id)
		if err != nil || template.EventID != "" || template.LatestPublishedVer == 0 {
			continue
		}
		out = append(out, templateDto.NewTemplateRes(template))
	}
	return out, nil
}

func (s *TemplateService) ListEventTemplates(ctx context.Context, eventID, kind string) ([]templateDto.TemplateRes, error) {
	templates, err := s.repo.ListTemplatesByEvent(ctx, eventID, kind)
	if err != nil {
		return nil, err
	}
	out := make([]templateDto.TemplateRes, 0, len(templates))
	for _, template := range templates {
		res := templateDto.NewTemplateRes(template)
		versionNo := template.LatestPublishedVer
		if versionNo <= 0 {
			versionNo = template.CurrentDraftVer
		}
		if versionNo > 0 {
			if version, err := s.repo.FindVersion(ctx, template.ID.Hex(), versionNo); err == nil {
				res.Style = version.Schema.Style
			}
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *TemplateService) validateStyle(style models.TemplateStyle) error {
	for _, color := range []string{style.Paper, style.Ink, style.Accent} {
		if !hexColorPattern.MatchString(color) {
			return fmt.Errorf("%w: style colors must be hex values", ErrInvalidTemplateSchema)
		}
	}
	return nil
}

func defaultColor(value, fallback string) string {
	if hexColorPattern.MatchString(strings.TrimSpace(value)) {
		return strings.TrimSpace(value)
	}
	return fallback
}

func (s *TemplateService) ListTemplates(ctx context.Context) ([]templateDto.TemplateRes, error) {
	templates, err := s.repo.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]templateDto.TemplateRes, 0, len(templates))
	for _, template := range templates {
		out = append(out, templateDto.NewTemplateRes(template))
	}
	return out, nil
}

func (s *TemplateService) GetTemplate(ctx context.Context, id string) (templateDto.TemplateDetailRes, error) {
	template, err := s.repo.FindTemplateByID(ctx, id)
	if err != nil {
		return templateDto.TemplateDetailRes{}, ErrTemplateNotFound
	}
	versions, err := s.repo.ListVersions(ctx, id)
	if err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	res := templateDto.TemplateDetailRes{
		TemplateRes: templateDto.NewTemplateRes(template),
		Versions:    make([]templateDto.TemplateVersionRes, 0, len(versions)),
	}
	for _, version := range versions {
		versionRes := templateDto.NewTemplateVersionRes(version)
		res.Versions = append(res.Versions, versionRes)
		if version.Version == template.CurrentDraftVer {
			res.Draft = versionRes
		}
	}
	if res.Draft.Version == 0 {
		return templateDto.TemplateDetailRes{}, ErrTemplateVersionNotFound
	}
	return res, nil
}

func (s *TemplateService) SaveDraft(ctx context.Context, id string, schema models.TemplateSchema) (templateDto.TemplateDetailRes, error) {
	if err := s.ValidateSchema(ctx, schema); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	template, err := s.repo.FindTemplateByID(ctx, id)
	if err != nil {
		return templateDto.TemplateDetailRes{}, ErrTemplateNotFound
	}
	version, err := s.repo.FindVersion(ctx, id, template.CurrentDraftVer)
	if err != nil {
		return templateDto.TemplateDetailRes{}, ErrTemplateVersionNotFound
	}
	version.Schema = schema
	version.UpdatedAt = time.Now()
	if err := s.repo.UpdateVersion(ctx, version); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	template.UpdatedAt = time.Now()
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	return s.GetTemplate(ctx, id)
}

func (s *TemplateService) Publish(ctx context.Context, id string) (templateDto.TemplateDetailRes, error) {
	template, err := s.repo.FindTemplateByID(ctx, id)
	if err != nil {
		return templateDto.TemplateDetailRes{}, ErrTemplateNotFound
	}
	draft, err := s.repo.FindVersion(ctx, id, template.CurrentDraftVer)
	if err != nil {
		return templateDto.TemplateDetailRes{}, ErrTemplateVersionNotFound
	}
	if err := s.ValidateSchema(ctx, draft.Schema); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}

	now := time.Now()
	draft.Status = "published"
	draft.UpdatedAt = now
	if err := s.repo.UpdateVersion(ctx, draft); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}

	nextDraft := models.TemplateVersion{
		ID:         primitive.NewObjectID(),
		TemplateID: id,
		Version:    template.CurrentDraftVer + 1,
		Status:     "draft",
		Schema:     draft.Schema,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if _, err := s.repo.InsertVersion(ctx, nextDraft); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}

	template.LatestPublishedVer = draft.Version
	template.CurrentDraftVer = nextDraft.Version
	template.UpdatedAt = now
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return templateDto.TemplateDetailRes{}, err
	}
	return s.GetTemplate(ctx, id)
}

func (s *TemplateService) ValidateAssignment(ctx context.Context, templateID string, version int) (int, error) {
	templateID = strings.TrimSpace(templateID)
	if templateID == "" {
		return 0, nil
	}
	template, err := s.repo.FindTemplateByID(ctx, templateID)
	if err != nil {
		return 0, ErrTemplateNotFound
	}
	if version <= 0 {
		version = template.LatestPublishedVer
	}
	versionDoc, err := s.repo.FindVersion(ctx, templateID, version)
	if err != nil || versionDoc.Status != "published" {
		return 0, ErrTemplateAssignment
	}
	return version, nil
}

func (s *TemplateService) ValidateSchema(ctx context.Context, schema models.TemplateSchema) error {
	if schema.Canvas.Width < 320 || schema.Canvas.Width > 4096 || schema.Canvas.Height < 320 || schema.Canvas.Height > 4096 {
		return fmt.Errorf("%w: canvas dimensions must be between 320 and 4096", ErrInvalidTemplateSchema)
	}
	if schema.Canvas.Background != "" && !hexColorPattern.MatchString(schema.Canvas.Background) {
		return fmt.Errorf("%w: canvas background must be a hex color", ErrInvalidTemplateSchema)
	}
	if len(schema.PhotoSlots) > 0 {
		if len(schema.PhotoSlots) != 4 {
			return fmt.Errorf("%w: strip templates require four photo slots", ErrInvalidTemplateSchema)
		}
		for _, slot := range schema.PhotoSlots {
			if err := validateSlot("photo slot", schema.Canvas.Width, schema.Canvas.Height, slot.X, slot.Y, slot.Width, slot.Height); err != nil {
				return err
			}
			if slot.Rotation < -360 || slot.Rotation > 360 {
				return fmt.Errorf("%w: photo slot rotation out of range", ErrInvalidTemplateSchema)
			}
		}
	} else {
		if err := validateSlot("photo slot", schema.Canvas.Width, schema.Canvas.Height, schema.PhotoSlot.X, schema.PhotoSlot.Y, schema.PhotoSlot.Width, schema.PhotoSlot.Height); err != nil {
			return err
		}
		if schema.PhotoSlot.Rotation < -360 || schema.PhotoSlot.Rotation > 360 {
			return fmt.Errorf("%w: photo slot rotation out of range", ErrInvalidTemplateSchema)
		}
	}
	if len(schema.Layers) > maxTemplateLayers {
		return fmt.Errorf("%w: too many layers", ErrInvalidTemplateSchema)
	}
	seen := make(map[string]bool, len(schema.Layers))
	for _, layer := range schema.Layers {
		if layer.ID == "" || seen[layer.ID] {
			return fmt.Errorf("%w: layer ids must be unique", ErrInvalidTemplateSchema)
		}
		seen[layer.ID] = true
		if layer.Width <= 0 || layer.Height <= 0 {
			return fmt.Errorf("%w: layer size must be positive", ErrInvalidTemplateSchema)
		}
		if layer.X < -float64(schema.Canvas.Width) || layer.Y < -float64(schema.Canvas.Height) ||
			layer.X > float64(schema.Canvas.Width)*2 || layer.Y > float64(schema.Canvas.Height)*2 {
			return fmt.Errorf("%w: layer position out of range", ErrInvalidTemplateSchema)
		}
		if layer.Rotation < -360 || layer.Rotation > 360 {
			return fmt.Errorf("%w: layer rotation out of range", ErrInvalidTemplateSchema)
		}
		switch layer.Type {
		case models.TemplateLayerImage:
			if strings.TrimSpace(layer.AssetID) == "" {
				return fmt.Errorf("%w: image layer requires an asset", ErrInvalidTemplateSchema)
			}
			asset, err := s.assets.FindByID(ctx, layer.AssetID)
			if err != nil {
				return fmt.Errorf("%w: image layer asset not found", ErrInvalidTemplateSchema)
			}
			if asset.Kind == models.AssetKindTexture {
				return fmt.Errorf("%w: texture assets cannot be template layers", ErrInvalidTemplateSchema)
			}
		case models.TemplateLayerText:
			if strings.TrimSpace(layer.Text) == "" || len(layer.Text) > 120 {
				return fmt.Errorf("%w: text layer requires text up to 120 characters", ErrInvalidTemplateSchema)
			}
		default:
			return fmt.Errorf("%w: unsupported layer type", ErrInvalidTemplateSchema)
		}
	}
	return nil
}

func DefaultTemplateSchema() models.TemplateSchema {
	return models.TemplateSchema{
		Canvas: models.TemplateCanvas{
			Width:      1080,
			Height:     1350,
			Background: "#F3EBD8",
		},
		PhotoSlot: models.TemplatePhotoSlot{
			X:      90,
			Y:      180,
			Width:  900,
			Height: 900,
		},
		Layers: []models.TemplateLayer{},
	}
}

func validateSlot(name string, canvasWidth, canvasHeight int, x, y, width, height float64) error {
	if width <= 0 || height <= 0 {
		return fmt.Errorf("%w: %s size must be positive", ErrInvalidTemplateSchema, name)
	}
	if x < 0 || y < 0 || x+width > float64(canvasWidth) || y+height > float64(canvasHeight) {
		return fmt.Errorf("%w: %s must fit inside the canvas", ErrInvalidTemplateSchema, name)
	}
	return nil
}
