package repositories

import (
	"context"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type CustomerRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, customer models.Customer) (string, error)
	FindByEmail(ctx context.Context, email string) (models.Customer, error)
	FindByID(ctx context.Context, id string) (models.Customer, error)
}

type CustomerSessionRepository interface {
	EnsureIndexes(ctx context.Context) error
	Insert(ctx context.Context, session models.CustomerSession) error
	FindByTokenHash(ctx context.Context, tokenHash string) (models.CustomerSession, error)
	DeleteByTokenHash(ctx context.Context, tokenHash string) error
}

type EventPlanRepository interface {
	EnsureIndexes(ctx context.Context) error
	SeedDefaults(ctx context.Context, plans []models.EventPlan) error
	List(ctx context.Context) ([]models.EventPlan, error)
	FindByCode(ctx context.Context, code string) (models.EventPlan, error)
	Update(ctx context.Context, plan models.EventPlan) error
}
