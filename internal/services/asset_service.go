package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	assetDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/assets"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/helper"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrInvalidAssetKind = errors.New("invalid asset kind")
	ErrInvalidAssetKey  = errors.New("asset object key is invalid")
)

type AssetService struct {
	repo         repositories.AssetRepository
	obj          storage.ObjectStorage
	bucket       string
	presignedTTL time.Duration
}

func NewAssetService(repo repositories.AssetRepository, obj storage.ObjectStorage, bucket string, presignedTTL time.Duration) *AssetService {
	return &AssetService{repo: repo, obj: obj, bucket: bucket, presignedTTL: presignedTTL}
}

func (s *AssetService) GeneratePresignedUpload(ctx context.Context, req assetDto.PresignAssetUploadReq) (assetDto.PresignAssetUploadRes, error) {
	kind, err := parseAssetKind(req.Kind)
	if err != nil {
		return assetDto.PresignAssetUploadRes{}, err
	}
	ext := helper.ExtractMimeType(req.MimeType)
	if ext == "" {
		return assetDto.PresignAssetUploadRes{}, fmt.Errorf("%w: %q", ErrUnsupportedMediaType, req.MimeType)
	}
	key := fmt.Sprintf("assets/%s-%s%s", string(kind), uuid.NewString(), ext)
	url, err := s.obj.PresignPut(ctx, s.bucket, key, s.presignedTTL)
	if err != nil {
		return assetDto.PresignAssetUploadRes{}, err
	}
	return assetDto.PresignAssetUploadRes{UploadUrl: url, ObjectKey: key, ContentType: req.MimeType}, nil
}

func (s *AssetService) GenerateEventPresignedUpload(ctx context.Context, eventID string, req assetDto.PresignAssetUploadReq) (assetDto.PresignAssetUploadRes, error) {
	kind, err := parseAssetKind(req.Kind)
	if err != nil {
		return assetDto.PresignAssetUploadRes{}, err
	}
	ext := helper.ExtractMimeType(req.MimeType)
	if ext == "" {
		return assetDto.PresignAssetUploadRes{}, fmt.Errorf("%w: %q", ErrUnsupportedMediaType, req.MimeType)
	}
	key := fmt.Sprintf("events/%s/assets/%s-%s%s", eventID, string(kind), uuid.NewString(), ext)
	url, err := s.obj.PresignPut(ctx, s.bucket, key, s.presignedTTL)
	if err != nil {
		return assetDto.PresignAssetUploadRes{}, err
	}
	return assetDto.PresignAssetUploadRes{UploadUrl: url, ObjectKey: key, ContentType: req.MimeType}, nil
}

func (s *AssetService) CreateEventAsset(ctx context.Context, eventID, ownerID string, req assetDto.CreateAssetReq) (assetDto.AssetRes, error) {
	kind, err := parseAssetKind(req.Kind)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	if !strings.HasPrefix(req.ObjectKey, "events/"+eventID+"/assets/") {
		return assetDto.AssetRes{}, ErrInvalidAssetKey
	}
	exists, err := s.obj.Exists(ctx, s.bucket, req.ObjectKey)
	if err != nil {
		return assetDto.AssetRes{}, fmt.Errorf("verify uploaded asset: %w", err)
	}
	if !exists {
		return assetDto.AssetRes{}, ErrUploadedObjectNotFound
	}
	now := time.Now()
	asset := models.Asset{
		ID:        primitive.NewObjectID(),
		OwnerType: string(models.EventOwnerCustomer),
		OwnerID:   ownerID,
		EventID:   eventID,
		Kind:      kind,
		Name:      strings.TrimSpace(req.Name),
		ObjectKey: req.ObjectKey,
		MimeType:  req.MimeType,
		Bytes:     req.Bytes,
		Width:     req.Width,
		Height:    req.Height,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if asset.Name == "" {
		return assetDto.AssetRes{}, errors.New("asset name is required")
	}
	id, err := s.repo.Insert(ctx, asset)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	asset.ID, _ = primitive.ObjectIDFromHex(id)
	return s.toRes(ctx, asset)
}

func (s *AssetService) ListEventAssets(ctx context.Context, eventID string) ([]assetDto.AssetRes, error) {
	assets, err := s.repo.ListByEvent(ctx, eventID, string(models.AssetKindSticker))
	if err != nil {
		return nil, err
	}
	out := make([]assetDto.AssetRes, 0, len(assets))
	for _, asset := range assets {
		res, err := s.toRes(ctx, asset)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *AssetService) ListPlatformAssets(ctx context.Context, kind string) ([]assetDto.AssetRes, error) {
	assets, err := s.repo.ListPlatform(ctx, kind)
	if err != nil {
		return nil, err
	}
	out := make([]assetDto.AssetRes, 0, len(assets))
	for _, asset := range assets {
		if asset.Disabled {
			continue
		}
		res, err := s.toRes(ctx, asset)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *AssetService) SetDisabled(ctx context.Context, id string, disabled bool) (assetDto.AssetRes, error) {
	return s.UpdateAsset(ctx, id, "", &disabled)
}

func (s *AssetService) UpdateAsset(ctx context.Context, id, name string, disabled *bool) (assetDto.AssetRes, error) {
	asset, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return assetDto.AssetRes{}, ErrAssetNotFound
	}
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		asset.Name = trimmed[:min(60, len(trimmed))]
	}
	if disabled != nil {
		asset.Disabled = *disabled
	}
	if err := s.repo.Update(ctx, asset); err != nil {
		return assetDto.AssetRes{}, err
	}
	return s.toRes(ctx, asset)
}

func (s *AssetService) UpdateEventAsset(ctx context.Context, eventID, assetID, name string) (assetDto.AssetRes, error) {
	asset, err := s.repo.FindByID(ctx, assetID)
	if err != nil || asset.EventID != eventID {
		return assetDto.AssetRes{}, ErrAssetNotFound
	}
	return s.UpdateAsset(ctx, assetID, name, nil)
}

func (s *AssetService) CreateAsset(ctx context.Context, req assetDto.CreateAssetReq) (assetDto.AssetRes, error) {
	kind, err := parseAssetKind(req.Kind)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	if !strings.HasPrefix(req.ObjectKey, "assets/") {
		return assetDto.AssetRes{}, ErrInvalidAssetKey
	}
	exists, err := s.obj.Exists(ctx, s.bucket, req.ObjectKey)
	if err != nil {
		return assetDto.AssetRes{}, fmt.Errorf("verify uploaded asset: %w", err)
	}
	if !exists {
		return assetDto.AssetRes{}, ErrUploadedObjectNotFound
	}
	now := time.Now()
	asset := models.Asset{
		ID:          primitive.NewObjectID(),
		Kind:        kind,
		Name:        strings.TrimSpace(req.Name),
		ObjectKey:   req.ObjectKey,
		MimeType:    req.MimeType,
		License:     strings.TrimSpace(req.License),
		Source:      strings.TrimSpace(req.Source),
		Attribution: strings.TrimSpace(req.Attribution),
		Bytes:       req.Bytes,
		Width:       req.Width,
		Height:      req.Height,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if asset.Name == "" {
		return assetDto.AssetRes{}, errors.New("asset name is required")
	}
	id, err := s.repo.Insert(ctx, asset)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	asset.ID, _ = primitive.ObjectIDFromHex(id)
	return s.toRes(ctx, asset)
}

func (s *AssetService) ListAssets(ctx context.Context, kind string) ([]assetDto.AssetRes, error) {
	parsedKind := models.AssetKind(kind)
	if kind != "" {
		var err error
		parsedKind, err = parseAssetKind(kind)
		if err != nil {
			return nil, err
		}
	}
	assets, err := s.repo.List(ctx, string(parsedKind))
	if err != nil {
		return nil, err
	}
	out := make([]assetDto.AssetRes, 0, len(assets))
	for _, asset := range assets {
		res, err := s.toRes(ctx, asset)
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *AssetService) PresignAsset(ctx context.Context, id string) (assetDto.AssetRes, error) {
	asset, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	return s.toRes(ctx, asset)
}

func (s *AssetService) GetAsset(ctx context.Context, id string) (models.Asset, error) {
	asset, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return models.Asset{}, ErrAssetNotFound
	}
	return asset, nil
}

var ErrAssetNotFound = errors.New("asset not found")

func (s *AssetService) toRes(ctx context.Context, asset models.Asset) (assetDto.AssetRes, error) {
	url, err := s.obj.PresignGet(ctx, s.bucket, asset.ObjectKey, s.presignedTTL)
	if err != nil {
		return assetDto.AssetRes{}, err
	}
	return assetDto.AssetRes{
		ID:          asset.ID.Hex(),
		OwnerType:   asset.OwnerType,
		OwnerID:     asset.OwnerID,
		EventID:     asset.EventID,
		Disabled:    asset.Disabled,
		Kind:        string(asset.Kind),
		Name:        asset.Name,
		ObjectKey:   asset.ObjectKey,
		Url:         url,
		MimeType:    asset.MimeType,
		License:     asset.License,
		Source:      asset.Source,
		Attribution: asset.Attribution,
		Bytes:       asset.Bytes,
		Width:       asset.Width,
		Height:      asset.Height,
	}, nil
}

func parseAssetKind(value string) (models.AssetKind, error) {
	switch models.AssetKind(value) {
	case models.AssetKindSticker, models.AssetKindBackground, models.AssetKindLogo, models.AssetKindTexture, models.AssetKindTemplate:
		return models.AssetKind(value), nil
	default:
		return "", ErrInvalidAssetKind
	}
}
