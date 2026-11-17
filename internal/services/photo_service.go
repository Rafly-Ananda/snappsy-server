package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	photoDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/helper"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrPhotoIntentNotFound        = errors.New("photo upload intent not found")
	ErrPhotoIntentExpired         = errors.New("photo upload intent expired")
	ErrPhotoStatusTokenInvalid    = errors.New("photo status token is invalid")
	ErrPhotoDownloadNotAllowed    = errors.New("downloads are not enabled for this event")
	ErrInvalidModerationState     = errors.New("moderation state must be pending, approved or hidden")
	ErrPhotoConsentRequired       = errors.New("consent is required")
	ErrPhotoIntentAlreadyComplete = errors.New("photo upload intent is already complete")
)

type PhotoService struct {
	media        repositories.MediaRepository
	intents      repositories.UploadIntentRepository
	events       repositories.EventRepository
	templates    repositories.TemplateRepository
	assets       repositories.AssetRepository
	obj          storage.ObjectStorage
	bucket       string
	presignedTTL time.Duration
	websocketHub *websocket.Hub
	drive        DriveEnqueuer
}

func NewPhotoService(
	media repositories.MediaRepository,
	intents repositories.UploadIntentRepository,
	events repositories.EventRepository,
	templates repositories.TemplateRepository,
	assets repositories.AssetRepository,
	obj storage.ObjectStorage,
	bucket string,
	presignedTTL time.Duration,
	hub *websocket.Hub,
	drive ...DriveEnqueuer,
) *PhotoService {
	service := &PhotoService{
		media:        media,
		intents:      intents,
		events:       events,
		templates:    templates,
		assets:       assets,
		obj:          obj,
		bucket:       bucket,
		presignedTTL: presignedTTL,
		websocketHub: hub,
	}
	if len(drive) > 0 {
		service.drive = drive[0]
	}
	return service
}

func (s *PhotoService) CreateIntent(ctx context.Context, req photoDto.CreateIntentReq) (photoDto.CreateIntentRes, error) {
	event, err := s.findUploadEvent(ctx, req.EventID)
	if err != nil {
		return photoDto.CreateIntentRes{}, err
	}
	if !isPhotoMime(req.MimeType) {
		return photoDto.CreateIntentRes{}, fmt.Errorf("%w: %q", ErrUnsupportedMediaType, req.MimeType)
	}

	id := uuid.NewString()
	prefix := fmt.Sprintf("events/%s/%s", safeObjectKeyPart(event.ID.Hex()), id)
	baseKey := prefix + "/base.jpg"
	compositeKey := prefix + "/composite.jpg"
	thumbnailKey := prefix + "/thumbnail.jpg"
	statusToken, statusHash, err := identity.NewDisplayToken()
	if err != nil {
		return photoDto.CreateIntentRes{}, err
	}
	expiresAt := time.Now().Add(s.presignedTTL)
	intent := models.UploadIntent{
		ID:              primitive.NewObjectID(),
		EventID:         event.ID.Hex(),
		BaseKey:         baseKey,
		CompositeKey:    compositeKey,
		ThumbnailKey:    thumbnailKey,
		StatusTokenHash: statusHash,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now(),
	}
	if _, err := s.intents.Insert(ctx, intent); err != nil {
		return photoDto.CreateIntentRes{}, err
	}

	baseURL, err := s.obj.PresignPut(ctx, s.bucket, baseKey, s.presignedTTL)
	if err != nil {
		return photoDto.CreateIntentRes{}, err
	}
	compositeURL, err := s.obj.PresignPut(ctx, s.bucket, compositeKey, s.presignedTTL)
	if err != nil {
		return photoDto.CreateIntentRes{}, err
	}
	thumbnailURL, err := s.obj.PresignPut(ctx, s.bucket, thumbnailKey, s.presignedTTL)
	if err != nil {
		return photoDto.CreateIntentRes{}, err
	}

	return photoDto.CreateIntentRes{
		IntentID:     intent.ID.Hex(),
		StatusToken:  statusToken,
		BaseUrl:      baseURL,
		CompositeUrl: compositeURL,
		ThumbnailUrl: thumbnailURL,
		BaseKey:      baseKey,
		CompositeKey: compositeKey,
		ThumbnailKey: thumbnailKey,
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *PhotoService) Commit(ctx context.Context, req photoDto.CommitReq) (photoDto.CommitRes, error) {
	if !req.Consent {
		return photoDto.CommitRes{}, ErrPhotoConsentRequired
	}
	intent, err := s.intents.FindByID(ctx, req.IntentID)
	if err != nil {
		return photoDto.CommitRes{}, ErrPhotoIntentNotFound
	}
	if time.Now().After(intent.ExpiresAt) {
		return photoDto.CommitRes{}, ErrPhotoIntentExpired
	}
	if subtle.ConstantTimeCompare([]byte(intent.StatusTokenHash), []byte(identity.HashToken(req.StatusToken))) != 1 {
		return photoDto.CommitRes{}, ErrPhotoStatusTokenInvalid
	}

	if existing, err := s.media.FindByUploadIntentID(ctx, req.IntentID); err == nil {
		return photoDto.CommitRes{
			ID:              existing.ID.Hex(),
			ProcessingState: existing.ProcessingState,
			ModerationState: existing.ModerationState,
		}, nil
	}

	event, err := s.findUploadEvent(ctx, intent.EventID)
	if err != nil {
		return photoDto.CommitRes{}, err
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || len(username) > 80 {
		return photoDto.CommitRes{}, errors.New("username must be between 1 and 80 characters")
	}
	if len(req.Captions) > 280 || len(req.GuestEditState) > 20000 {
		return photoDto.CommitRes{}, errors.New("caption or edit state is too large")
	}
	kind := models.MediaKindPhoto
	if req.Kind == string(models.MediaKindPhotoStrip) {
		kind = models.MediaKindPhotoStrip
	} else if req.Kind != "" && req.Kind != string(models.MediaKindPhoto) {
		return photoDto.CommitRes{}, errors.New("unsupported media kind")
	}
	if !event.AllowsCapture(kind) {
		return photoDto.CommitRes{}, ErrCaptureModeDisabled
	}
	for _, key := range []string{intent.BaseKey, intent.CompositeKey, intent.ThumbnailKey} {
		exists, err := s.obj.Exists(ctx, s.bucket, key)
		if err != nil {
			return photoDto.CommitRes{}, fmt.Errorf("verify uploaded photo: %w", err)
		}
		if !exists {
			return photoDto.CommitRes{}, ErrUploadedObjectNotFound
		}
	}

	if err := s.consumeQuota(ctx, event.ID.Hex(), event.UploadPolicy.AllowOverQuota); err != nil {
		return photoDto.CommitRes{}, err
	}

	now := time.Now()
	moderation := models.MediaModerationApproved
	if event.UploadPolicy.RequireApproval {
		moderation = models.MediaModerationPending
	}
	item := models.MediaItem{
		ID:              primitive.NewObjectID(),
		EventID:         event.ID.Hex(),
		Kind:            kind,
		Username:        username,
		GuestIDHash:     guestIdentityHash(req.GuestID, username),
		ObjectKey:       intent.CompositeKey,
		BaseKey:         intent.BaseKey,
		CompositeKey:    intent.CompositeKey,
		ThumbnailKey:    intent.ThumbnailKey,
		Captions:        strings.TrimSpace(req.Captions),
		Orientation:     strings.TrimSpace(req.Orientation),
		ProcessingState: models.MediaProcessingReady,
		ModerationState: moderation,
		TemplateID:      event.TemplateID,
		TemplateVersion: event.TemplateVersion,
		GuestEditState:  req.GuestEditState,
		ConsentAt:       &now,
		RetentionAt:     retentionDeadline(event.RetentionDays, now),
		StatusTokenHash: intent.StatusTokenHash,
		UploadIntentID:  req.IntentID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	id, err := s.media.Insert(ctx, item)
	if err != nil {
		if existing, findErr := s.media.FindByUploadIntentID(ctx, req.IntentID); findErr == nil {
			return photoDto.CommitRes{ID: existing.ID.Hex(), ProcessingState: existing.ProcessingState, ModerationState: existing.ModerationState}, nil
		}
		s.releaseQuota(ctx, event.ID.Hex())
		return photoDto.CommitRes{}, err
	}
	item.ID, _ = primitive.ObjectIDFromHex(id)
	if err := s.intents.Complete(ctx, req.IntentID, id); err != nil {
		return photoDto.CommitRes{}, err
	}
	if s.websocketHub != nil {
		s.websocketHub.Publish(item.EventID, websocket.MediaReadyMessage(item.EventID, id, string(item.Kind)))
	}
	if s.drive != nil {
		if err := s.drive.Enqueue(ctx, item); err != nil {
			log.Printf("drive enqueue photo %s: %v", id, err)
		}
	}
	return photoDto.CommitRes{ID: id, ProcessingState: item.ProcessingState, ModerationState: item.ModerationState}, nil
}

func (s *PhotoService) consumeQuota(ctx context.Context, eventID string, allowOverQuota bool) error {
	ok, err := s.events.ConsumeQuota(ctx, eventID, models.MediaKindPhoto, allowOverQuota)
	if err != nil {
		return err
	}
	if !ok {
		return ErrEventQuotaExceeded
	}
	return nil
}

func (s *PhotoService) releaseQuota(ctx context.Context, eventID string) {
	_ = s.events.ReleaseQuota(ctx, eventID, models.MediaKindPhoto)
}

func (s *PhotoService) Status(ctx context.Context, id string, statusToken string) (photoDto.StatusRes, error) {
	item, event, err := s.authorizedMedia(ctx, id, statusToken)
	if err != nil {
		return photoDto.StatusRes{}, err
	}
	res := photoDto.StatusRes{
		ID:              item.ID.Hex(),
		Kind:            item.Kind,
		ProcessingState: item.ProcessingState,
		ModerationState: item.ModerationState,
	}
	key := item.CompositeKey
	if key == "" {
		key = item.ObjectKey
	}
	if key != "" {
		url, err := s.obj.PresignGet(ctx, s.bucket, key, s.presignedTTL)
		if err != nil {
			return photoDto.StatusRes{}, err
		}
		res.URL = url
	}
	if event.UploadPolicy.AllowDownload && item.ProcessingState == models.MediaProcessingReady && key != "" {
		res.DownloadURL = res.URL
	}
	return res, nil
}

func (s *PhotoService) Download(ctx context.Context, id string, statusToken string) (string, error) {
	item, event, err := s.authorizedMedia(ctx, id, statusToken)
	if err != nil {
		return "", err
	}
	if !event.UploadPolicy.AllowDownload {
		return "", ErrPhotoDownloadNotAllowed
	}
	if item.ProcessingState != models.MediaProcessingReady {
		return "", ErrPhotoNotReady
	}
	key := item.CompositeKey
	if key == "" {
		key = item.ObjectKey
	}
	if key == "" {
		return "", ErrUploadedObjectNotFound
	}
	return s.obj.PresignGet(ctx, s.bucket, key, s.presignedTTL)
}

func (s *PhotoService) ListForEvent(ctx context.Context, eventID, cursor string, limit int, moderationState string) (photoDto.AdminMediaListRes, error) {
	if _, err := s.events.FindByID(ctx, eventID); err != nil {
		return photoDto.AdminMediaListRes{}, ErrEventNotFound
	}
	if limit <= 0 || limit > 60 {
		limit = 24
	}
	if moderationState != string(models.MediaModerationPending) && moderationState != string(models.MediaModerationApproved) && moderationState != string(models.MediaModerationHidden) {
		moderationState = ""
	}
	items, nextCursor, err := s.media.FindAllByEvents(ctx, eventID, moderationState, cursor, limit)
	if err != nil {
		return photoDto.AdminMediaListRes{}, err
	}
	out := make([]photoDto.AdminMediaRes, 0, len(items))
	for _, item := range items {
		key := item.ThumbnailKey
		if key == "" {
			key = item.CompositeKey
		}
		if key == "" {
			key = item.ObjectKey
		}
		url := ""
		if key != "" {
			url, _ = s.obj.PresignGet(ctx, s.bucket, key, s.presignedTTL)
		}
		playbackURL := ""
		if item.Kind == models.MediaKindBoomerang && item.OutputKey != "" {
			playbackURL, _ = s.obj.PresignGet(ctx, s.bucket, item.OutputKey, s.presignedTTL)
		}
		out = append(out, photoDto.AdminMediaRes{
			ID:              item.ID.Hex(),
			Kind:            item.Kind,
			Username:        item.Username,
			Captions:        item.Captions,
			ProcessingState: item.ProcessingState,
			ModerationState: item.ModerationState,
			URL:             url,
			PlaybackURL:     playbackURL,
			CreatedAt:       item.CreatedAt,
		})
	}
	return photoDto.AdminMediaListRes{Data: out, NextCursor: nextCursor}, nil
}

func (s *PhotoService) ListForCustomerEvent(ctx context.Context, eventID, ownerID, cursor string, limit int, moderationState string) (photoDto.AdminMediaListRes, error) {
	event, err := s.events.FindByID(ctx, eventID)
	if err != nil {
		return photoDto.AdminMediaListRes{}, ErrEventNotFound
	}
	if event.OwnerType != models.EventOwnerCustomer || event.OwnerID != ownerID {
		return photoDto.AdminMediaListRes{}, ErrEventNotFound
	}
	return s.ListForEvent(ctx, eventID, cursor, limit, moderationState)
}

func (s *PhotoService) SetModerationStateForOwner(ctx context.Context, id, ownerID, state string) error {
	item, err := s.media.FindByID(ctx, id)
	if err != nil {
		return ErrPhotoNotFound
	}
	event, err := s.events.FindByID(ctx, item.EventID)
	if err != nil {
		return ErrEventNotFound
	}
	if event.OwnerType != models.EventOwnerCustomer || event.OwnerID != ownerID {
		return ErrEventNotFound
	}
	return s.SetModerationState(ctx, id, state)
}

func (s *PhotoService) SetModerationState(ctx context.Context, id string, state string) error {
	moderation := models.MediaModerationState(state)
	switch moderation {
	case models.MediaModerationPending, models.MediaModerationApproved, models.MediaModerationHidden:
	default:
		return ErrInvalidModerationState
	}
	item, err := s.media.FindByID(ctx, id)
	if err != nil {
		return ErrPhotoNotFound
	}
	item.ModerationState = moderation
	item.UpdatedAt = time.Now()
	if err := s.media.Update(ctx, item); err != nil {
		return err
	}
	if s.websocketHub != nil {
		s.websocketHub.Publish(item.EventID, websocket.MediaChangedMessage(item.EventID, item.ID.Hex(), string(item.Kind)))
	}
	return nil
}

func (s *PhotoService) PublicDesign(ctx context.Context, slug string) (photoDto.DesignRes, error) {
	event, err := s.events.FindBySlug(ctx, slug)
	if err != nil || !event.IsLiveAt(time.Now()) {
		return photoDto.DesignRes{}, ErrEventNotFound
	}
	res := photoDto.DesignRes{
		EventID:         event.ID.Hex(),
		TemplateID:      event.TemplateID,
		TemplateVersion: event.TemplateVersion,
		Schema:          DefaultTemplateSchema(),
		Assets:          []photoDto.DesignAsset{},
	}
	seen := make(map[string]bool)
	appendAsset := func(asset models.Asset) {
		if seen[asset.ID.Hex()] {
			return
		}
		url, err := s.obj.PresignGet(ctx, s.bucket, asset.ObjectKey, s.presignedTTL)
		if err != nil {
			return
		}
		seen[asset.ID.Hex()] = true
		res.Assets = append(res.Assets, photoDto.DesignAsset{
			ID:        asset.ID.Hex(),
			Kind:      string(asset.Kind),
			Name:      asset.Name,
			ObjectKey: asset.ObjectKey,
			URL:       url,
		})
	}
	if event.TemplateID != "" {
		version, err := s.templates.FindVersion(ctx, event.TemplateID, event.TemplateVersion)
		if err != nil || version.Status != "published" {
			return photoDto.DesignRes{}, ErrTemplateNotFound
		}
		res.Schema = version.Schema
		for _, layer := range version.Schema.Layers {
			if layer.Type != models.TemplateLayerImage || layer.AssetID == "" {
				continue
			}
			asset, err := s.assets.FindByID(ctx, layer.AssetID)
			if err != nil {
				continue
			}
			appendAsset(asset)
		}
	}
	if event.StripTemplateID != "" {
		if version, err := s.templates.FindVersion(ctx, event.StripTemplateID, event.StripTemplateVersion); err == nil && version.Status == "published" {
			res.StripSchema = version.Schema
			res.StripStyle = version.Schema.Style
			for _, layer := range version.Schema.Layers {
				if layer.Type == models.TemplateLayerImage && layer.AssetID != "" {
					if asset, assetErr := s.assets.FindByID(ctx, layer.AssetID); assetErr == nil {
						appendAsset(asset)
					}
				}
			}
		}
	}
	if event.BoomerangTemplateID != "" {
		if version, err := s.templates.FindVersion(ctx, event.BoomerangTemplateID, event.BoomerangTemplateVersion); err == nil && version.Status == "published" {
			res.BoomerangSchema = version.Schema
			res.BoomerangStyle = version.Schema.Style
			for _, layer := range version.Schema.Layers {
				if layer.Type == models.TemplateLayerImage && layer.AssetID != "" {
					if asset, assetErr := s.assets.FindByID(ctx, layer.AssetID); assetErr == nil {
						appendAsset(asset)
					}
				}
			}
		}
	}
	hidden := make(map[string]bool, len(event.HiddenStickerIDs))
	for _, id := range event.HiddenStickerIDs {
		hidden[id] = true
	}
	allowed := make(map[string]bool, len(event.StickerAssetIDs))
	for _, id := range event.StickerAssetIDs {
		allowed[id] = true
	}
	stickers, err := s.assets.ListPlatform(ctx, string(models.AssetKindSticker))
	if err == nil {
		for _, sticker := range stickers {
			if sticker.Disabled || hidden[sticker.ID.Hex()] {
				continue
			}
			if len(allowed) > 0 && !allowed[sticker.ID.Hex()] {
				continue
			}
			appendAsset(sticker)
		}
	}
	if eventStickers, err := s.assets.ListByEvent(ctx, event.ID.Hex(), string(models.AssetKindSticker)); err == nil {
		for _, sticker := range eventStickers {
			if hidden[sticker.ID.Hex()] {
				continue
			}
			appendAsset(sticker)
		}
	}
	return res, nil
}

func (s *PhotoService) authorizedMedia(ctx context.Context, id string, statusToken string) (models.MediaItem, models.Event, error) {
	item, err := s.media.FindByID(ctx, id)
	if err != nil {
		return models.MediaItem{}, models.Event{}, ErrPhotoNotFound
	}
	if subtle.ConstantTimeCompare([]byte(item.StatusTokenHash), []byte(identity.HashToken(statusToken))) != 1 {
		return models.MediaItem{}, models.Event{}, ErrPhotoStatusTokenInvalid
	}
	event, err := s.events.FindByID(ctx, item.EventID)
	if err != nil {
		return models.MediaItem{}, models.Event{}, ErrEventNotFound
	}
	return item, event, nil
}

func (s *PhotoService) findUploadEvent(ctx context.Context, eventID string) (models.Event, error) {
	event, err := s.events.FindByID(ctx, eventID)
	if err != nil {
		return models.Event{}, ErrEventNotFound
	}
	if !event.IsLiveAt(time.Now()) || !event.UploadPolicy.AcceptUploads {
		return models.Event{}, ErrEventNotAcceptingUploads
	}
	return event, nil
}

var ErrPhotoNotFound = errors.New("media not found")
var ErrPhotoNotReady = errors.New("media is not ready")

func isPhotoMime(mimeType string) bool {
	switch mimeType {
	case "image/jpeg", "image/jpg", "image/png", "image/webp":
		return helper.ExtractMimeType(mimeType) != ""
	default:
		return false
	}
}
