package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	boomerangDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/boomerangs"
	photoDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/helper"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	ErrBoomerangQueueFull      = errors.New("boomerang processing queue is full for this event")
	ErrBoomerangSourceTooLarge = errors.New("boomerang source video is too large")
	ErrBoomerangInvalidSegment = errors.New("boomerang segment is invalid")
	ErrBoomerangInvalidSource  = errors.New("boomerang source must be MP4, WebM or MOV")
	ErrBoomerangStatusToken    = errors.New("boomerang status token is invalid")
)

type BoomerangService struct {
	media        repositories.MediaRepository
	intents      repositories.UploadIntentRepository
	jobs         repositories.MediaJobRepository
	events       repositories.EventRepository
	obj          storage.ObjectStorage
	bucket       string
	presignedTTL time.Duration
	segment      int
	maxBytes     int64
	queueLimit   int
}

func NewBoomerangService(
	media repositories.MediaRepository,
	intents repositories.UploadIntentRepository,
	jobs repositories.MediaJobRepository,
	events repositories.EventRepository,
	obj storage.ObjectStorage,
	bucket string,
	presignedTTL time.Duration,
	segmentSeconds int,
	maxBytes int64,
	queueLimit int,
) *BoomerangService {
	return &BoomerangService{
		media:        media,
		intents:      intents,
		jobs:         jobs,
		events:       events,
		obj:          obj,
		bucket:       bucket,
		presignedTTL: presignedTTL,
		segment:      segmentSeconds,
		maxBytes:     maxBytes,
		queueLimit:   queueLimit,
	}
}

func (s *BoomerangService) CreateIntent(ctx context.Context, req boomerangDto.CreateIntentReq) (boomerangDto.CreateIntentRes, error) {
	event, err := s.findUploadEvent(ctx, req.EventID)
	if err != nil {
		return boomerangDto.CreateIntentRes{}, err
	}
	if !event.AllowsCapture(models.MediaKindBoomerang) {
		return boomerangDto.CreateIntentRes{}, ErrCaptureModeDisabled
	}
	mimeType := strings.TrimSpace(strings.Split(req.MimeType, ";")[0])
	ext := helper.ExtractMimeType(mimeType)
	if ext != ".mp4" && ext != ".webm" && ext != ".mov" {
		return boomerangDto.CreateIntentRes{}, ErrBoomerangInvalidSource
	}
	if req.Bytes > 0 && req.Bytes > s.maxBytes {
		return boomerangDto.CreateIntentRes{}, ErrBoomerangSourceTooLarge
	}
	sourceKey := fmt.Sprintf("events/%s/boomerangs/%s/source%s", safeObjectKeyPart(event.ID.Hex()), uuid.NewString(), ext)
	statusToken, statusHash, err := identity.NewDisplayToken()
	if err != nil {
		return boomerangDto.CreateIntentRes{}, err
	}
	expiresAt := time.Now().Add(s.presignedTTL)
	intent := models.UploadIntent{
		ID:              primitive.NewObjectID(),
		EventID:         event.ID.Hex(),
		BaseKey:         sourceKey,
		StatusTokenHash: statusHash,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now(),
	}
	if _, err := s.intents.Insert(ctx, intent); err != nil {
		return boomerangDto.CreateIntentRes{}, err
	}
	url, err := s.obj.PresignPut(ctx, s.bucket, sourceKey, s.presignedTTL)
	if err != nil {
		return boomerangDto.CreateIntentRes{}, err
	}
	return boomerangDto.CreateIntentRes{
		IntentID:    intent.ID.Hex(),
		StatusToken: statusToken,
		SourceUrl:   url,
		SourceKey:   sourceKey,
		ExpiresAt:   expiresAt,
		MaxSeconds:  s.segment,
	}, nil
}

func (s *BoomerangService) QueueStatus(ctx context.Context, eventID string) (boomerangDto.QueueStatusRes, error) {
	active, failed, err := s.jobs.SummaryByEvent(ctx, eventID)
	if err != nil {
		return boomerangDto.QueueStatusRes{}, err
	}
	return boomerangDto.QueueStatusRes{Active: active, Failed: failed}, nil
}

func (s *BoomerangService) Commit(ctx context.Context, req boomerangDto.CommitReq) (photoDto.CommitRes, error) {
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
		return photoDto.CommitRes{}, ErrBoomerangStatusToken
	}
	event, err := s.findUploadEvent(ctx, intent.EventID)
	if err != nil {
		return photoDto.CommitRes{}, err
	}
	if !event.AllowsCapture(models.MediaKindBoomerang) {
		return photoDto.CommitRes{}, ErrCaptureModeDisabled
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || len(username) > 80 {
		return photoDto.CommitRes{}, errors.New("username must be between 1 and 80 characters")
	}
	if len(req.Captions) > 280 {
		return photoDto.CommitRes{}, errors.New("caption is too large")
	}
	if req.StartSeconds < 0 || req.SegmentSeconds <= 0 || req.SegmentSeconds > float64(s.segment) {
		return photoDto.CommitRes{}, ErrBoomerangInvalidSegment
	}
	exists, err := s.obj.Exists(ctx, s.bucket, intent.BaseKey)
	if err != nil {
		return photoDto.CommitRes{}, fmt.Errorf("verify uploaded boomerang source: %w", err)
	}
	if !exists {
		return photoDto.CommitRes{}, ErrUploadedObjectNotFound
	}
	if existing, err := s.media.FindByUploadIntentID(ctx, req.IntentID); err == nil {
		return photoDto.CommitRes{ID: existing.ID.Hex(), ProcessingState: existing.ProcessingState, ModerationState: existing.ModerationState}, nil
	}
	active, err := s.jobs.CountActiveByEvent(ctx, event.ID.Hex())
	if err != nil {
		return photoDto.CommitRes{}, err
	}
	if s.queueLimit > 0 && active >= int64(s.queueLimit) {
		return photoDto.CommitRes{}, ErrBoomerangQueueFull
	}

	if ok, err := s.events.ConsumeQuota(ctx, event.ID.Hex(), models.MediaKindBoomerang, event.UploadPolicy.AllowOverQuota); err != nil {
		return photoDto.CommitRes{}, err
	} else if !ok {
		return photoDto.CommitRes{}, ErrEventQuotaExceeded
	}

	now := time.Now()
	moderation := models.MediaModerationApproved
	if event.UploadPolicy.RequireApproval {
		moderation = models.MediaModerationPending
	}
	item := models.MediaItem{
		ID:              primitive.NewObjectID(),
		EventID:         event.ID.Hex(),
		Kind:            models.MediaKindBoomerang,
		Username:        username,
		GuestIDHash:     guestIdentityHash(req.GuestID, username),
		ObjectKey:       intent.BaseKey,
		SourceKey:       intent.BaseKey,
		Captions:        strings.TrimSpace(req.Captions),
		Orientation:     "portrait",
		ProcessingState: models.MediaProcessingProcessing,
		ModerationState: moderation,
		TemplateID:      event.TemplateID,
		TemplateVersion: event.TemplateVersion,
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
		_ = s.events.ReleaseQuota(ctx, event.ID.Hex(), models.MediaKindBoomerang)
		return photoDto.CommitRes{}, err
	}
	job := models.MediaJob{
		ID:             primitive.NewObjectID(),
		MediaID:        id,
		EventID:        event.ID.Hex(),
		SourceKey:      intent.BaseKey,
		State:          models.MediaJobQueued,
		StartSeconds:   req.StartSeconds,
		SegmentSeconds: req.SegmentSeconds,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.jobs.Insert(ctx, job); err != nil {
		item.ProcessingState = models.MediaProcessingFailed
		item.UpdatedAt = time.Now()
		_ = s.media.Update(ctx, item)
		_ = s.events.ReleaseQuota(ctx, event.ID.Hex(), models.MediaKindBoomerang)
		return photoDto.CommitRes{}, err
	}
	if err := s.intents.Complete(ctx, req.IntentID, id); err != nil {
		return photoDto.CommitRes{}, err
	}
	return photoDto.CommitRes{ID: id, ProcessingState: models.MediaProcessingProcessing, ModerationState: moderation}, nil
}

func (s *BoomerangService) Status(ctx context.Context, id string, statusToken string) (photoDto.StatusRes, error) {
	item, event, err := s.authorizedMedia(ctx, id, statusToken)
	if err != nil {
		return photoDto.StatusRes{}, err
	}
	res := photoDto.StatusRes{ID: item.ID.Hex(), Kind: item.Kind, ProcessingState: item.ProcessingState, ModerationState: item.ModerationState}
	previewKey := item.PosterKey
	if previewKey == "" {
		previewKey = item.CompositeKey
	}
	if previewKey == "" {
		previewKey = item.ObjectKey
	}
	if previewKey != "" {
		res.URL, _ = s.obj.PresignGet(ctx, s.bucket, previewKey, s.presignedTTL)
	}
	if item.ProcessingState == models.MediaProcessingReady && item.OutputKey != "" {
		res.PlaybackURL, _ = s.obj.PresignGet(ctx, s.bucket, item.OutputKey, s.presignedTTL)
	}
	if event.UploadPolicy.AllowDownload && item.ProcessingState == models.MediaProcessingReady {
		downloadKey := item.OutputKey
		if downloadKey == "" {
			downloadKey = item.ObjectKey
		}
		if downloadKey != "" {
			res.DownloadURL, _ = s.obj.PresignGet(ctx, s.bucket, downloadKey, s.presignedTTL)
		}
	}
	return res, nil
}

func (s *BoomerangService) Download(ctx context.Context, id string, statusToken string) (string, error) {
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
	key := item.OutputKey
	if key == "" {
		key = item.ObjectKey
	}
	if key == "" {
		return "", ErrUploadedObjectNotFound
	}
	return s.obj.PresignGet(ctx, s.bucket, key, s.presignedTTL)
}

func (s *BoomerangService) authorizedMedia(ctx context.Context, id string, statusToken string) (models.MediaItem, models.Event, error) {
	item, err := s.media.FindByID(ctx, id)
	if err != nil {
		return models.MediaItem{}, models.Event{}, ErrPhotoNotFound
	}
	if subtle.ConstantTimeCompare([]byte(item.StatusTokenHash), []byte(identity.HashToken(statusToken))) != 1 {
		return models.MediaItem{}, models.Event{}, ErrBoomerangStatusToken
	}
	event, err := s.events.FindByID(ctx, item.EventID)
	if err != nil {
		return models.MediaItem{}, models.Event{}, ErrEventNotFound
	}
	return item, event, nil
}

func (s *BoomerangService) findUploadEvent(ctx context.Context, eventID string) (models.Event, error) {
	event, err := s.events.FindByID(ctx, eventID)
	if err != nil {
		return models.Event{}, ErrEventNotFound
	}
	if !event.IsLiveAt(time.Now()) || !event.UploadPolicy.AcceptUploads {
		return models.Event{}, ErrEventNotAcceptingUploads
	}
	return event, nil
}
