package services

import (
	"context"
	"fmt"
	"time"

	imgDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/images"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/helper"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/storage"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ImageService struct {
	repo         repositories.ImageRepository
	obj          storage.ObjectStorage
	bucket       string
	presignedTtl time.Duration
	websocketHub *websocket.Hub
}

func NewImageService(
	repo repositories.ImageRepository,
	obj storage.ObjectStorage,
	bucket string,
	presignedTtl time.Duration,
	websocketHub *websocket.Hub,
	) *ImageService {
	return &ImageService{
		repo:         repo,
		obj:          obj,
		bucket:       bucket,
		presignedTtl: presignedTtl,
		websocketHub: websocketHub, 
	}
}

func (s *ImageService) CommitImageUpload(ctx context.Context, req imgDto.CommitUploadReq) (imgDto.CommitUploadRes, error) {
	image := models.Images{
		ID:        primitive.NewObjectID(),
		EventId:   req.EventId,
		Username:  req.Username,
		MinioKey:  req.MinioKey,
		Captions:  req.Captions,
		Orientation: req.Orientation,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	id, err := s.repo.Insert(ctx, image)
	if err != nil {
		return imgDto.CommitUploadRes{}, err
	}

	
	s.websocketHub.Broadcast("New Image Added to Event " + id)

	return imgDto.CommitUploadRes{ID: id}, nil
}

func (s *ImageService) GeneratePresignedUploader(ctx context.Context, req imgDto.GeneratePresignedUrlReq) (imgDto.GeneratePresignedUrlRes, error) {
	// Check MimeType
	mime := helper.ExtractMimeType(req.MimeType)

	// Generate unique object key (combine username, session)
	timeStamp := time.Now().Format("20060102150405")

	key := fmt.Sprintf("%s-%s-%s%s", req.Username, req.EventId, timeStamp, mime)

	// Get presigned upload URL from storage (via the interface)
	url, err := s.obj.PresignPut(ctx, s.bucket, key, s.presignedTtl)
	if err != nil {
		fmt.Println("Error here")
		fmt.Println(err)
		return imgDto.GeneratePresignedUrlRes{}, err
	}

	return imgDto.GeneratePresignedUrlRes{
		UploadUrl:   url,
		ObjectKey:   key,
		ContentType: req.MimeType,
	}, nil
}

func (s *ImageService) GeneratePresignedViewer(ctx context.Context, key string, expiry time.Duration) (imgDto.GeneratePresignedUrlView, error) {
	url, err := s.obj.PresignGet(ctx, s.bucket, key, expiry)
	if err != nil {
		return imgDto.GeneratePresignedUrlView{}, err
	}

	return imgDto.GeneratePresignedUrlView{
		Url: url,
	}, nil
}

func (s *ImageService) GetAllPresignedImagesByEvent(ctx context.Context, eventId string, cursor string, limit int, expiry time.Duration) ([]imgDto.GeneratePresignedUrlView, string, error) {
	imgs, next, err := s.repo.FindAllByEvents(ctx, eventId, cursor, limit)
	if err != nil {
		return []imgDto.GeneratePresignedUrlView{}, "", err
	}

	var presignedList []imgDto.GeneratePresignedUrlView

	for _, item := range imgs {
		url, err := s.obj.PresignGet(ctx, s.bucket, item.MinioKey, expiry)
		if err != nil {
			continue
		}

		presignedList = append(presignedList, imgDto.GeneratePresignedUrlView{
			Id:       item.ID.Hex(),
			Url:      url,
			Captions: item.Captions,
			From:     item.Username,
			Orientation: item.Orientation,
			CreatedAt:  item.CreatedAt.String(),
		})
	}

	return presignedList, next, nil
}
