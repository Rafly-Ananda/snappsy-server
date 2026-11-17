package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var ErrPlanNotFound = errors.New("event plan not found")

type PlanService struct {
	plans repositories.EventPlanRepository
}

func NewPlanService(plans repositories.EventPlanRepository) *PlanService {
	return &PlanService{plans: plans}
}

func DefaultEventPlans() []models.EventPlan {
	now := time.Now()
	return []models.EventPlan{
		{
			ID: primitive.NewObjectID(), Code: "trial", Name: "Trial",
			Description: "Try a small event with a short retention window.",
			Price:       0, Currency: "IDR", PhotoQuota: 25, BoomerangQuota: 5,
			RetentionDays: 7, Active: true, SortOrder: 1, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Code: "starter", Name: "Starter",
			Description: "A focused event with room for every guest.",
			Price:       99000, Currency: "IDR", PhotoQuota: 100, BoomerangQuota: 20,
			RetentionDays: 30, Active: true, SortOrder: 2, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Code: "celebration", Name: "Celebration",
			Description: "The usual choice for a full celebration.",
			Price:       249000, Currency: "IDR", PhotoQuota: 400, BoomerangQuota: 75,
			RetentionDays: 60, Active: true, SortOrder: 3, CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: primitive.NewObjectID(), Code: "big_day", Name: "Big Day",
			Description: "High-volume events with the longest retention.",
			Price:       499000, Currency: "IDR", PhotoQuota: 1200, BoomerangQuota: 200,
			RetentionDays: 90, Active: true, SortOrder: 4, CreatedAt: now, UpdatedAt: now,
		},
	}
}

func (s *PlanService) Seed(ctx context.Context) error {
	return s.plans.SeedDefaults(ctx, DefaultEventPlans())
}

func (s *PlanService) List(ctx context.Context) ([]models.EventPlan, error) {
	return s.plans.List(ctx)
}

func (s *PlanService) Find(ctx context.Context, code string) (models.EventPlan, error) {
	plan, err := s.plans.FindByCode(ctx, strings.TrimSpace(code))
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.EventPlan{}, ErrPlanNotFound
		}
		return models.EventPlan{}, err
	}
	if !plan.Active {
		return models.EventPlan{}, ErrPlanNotFound
	}
	return plan, nil
}

func (s *PlanService) Update(ctx context.Context, plan models.EventPlan) (models.EventPlan, error) {
	existing, err := s.plans.FindByCode(ctx, plan.Code)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return models.EventPlan{}, ErrPlanNotFound
		}
		return models.EventPlan{}, err
	}
	if plan.Name != "" {
		existing.Name = plan.Name
	}
	if plan.Description != "" {
		existing.Description = plan.Description
	}
	if plan.Price >= 0 {
		existing.Price = plan.Price
	}
	if plan.Currency != "" {
		existing.Currency = plan.Currency
	}
	if plan.PhotoQuota >= -1 {
		existing.PhotoQuota = plan.PhotoQuota
	}
	if plan.BoomerangQuota >= -1 {
		existing.BoomerangQuota = plan.BoomerangQuota
	}
	if plan.RetentionDays > 0 {
		existing.RetentionDays = plan.RetentionDays
	}
	existing.Active = plan.Active
	existing.UpdatedAt = time.Now()
	if err := s.plans.Update(ctx, existing); err != nil {
		return models.EventPlan{}, err
	}
	return existing, nil
}
