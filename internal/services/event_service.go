package services

import (
	"context"

	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EventService struct {
	repo repositories.EventRepository
}

func NewEventService(repo repositories.EventRepository) *EventService {
	return &EventService{
		repo: repo,
	}
}

func (s *EventService) RegisterEvent(ctx context.Context, req eventDto.CreateEventReq) (eventDto.CreateEventRes, error) {
	event := models.Events{
		ID:          primitive.NewObjectID(),
		EventName:   req.EventName,
		Description: req.Description,
	}

	id, err := s.repo.Insert(ctx, event)
	if err != nil {
		return eventDto.CreateEventRes{}, err
	}

	return eventDto.CreateEventRes{ID: id}, nil
}
