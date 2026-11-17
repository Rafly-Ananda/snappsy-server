package customer

import (
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type SignupReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginReq struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type UserRes struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuthRes struct {
	User UserRes `json:"user"`
}

type PlanRes struct {
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Price          int64  `json:"price"`
	Currency       string `json:"currency"`
	PhotoQuota     int    `json:"photoQuota"`
	BoomerangQuota int    `json:"boomerangQuota"`
	RetentionDays  int    `json:"retentionDays"`
}

func NewUserRes(customer models.Customer) UserRes {
	return UserRes{
		ID:        customer.ID.Hex(),
		Email:     customer.Email,
		CreatedAt: customer.CreatedAt,
		UpdatedAt: customer.UpdatedAt,
	}
}

func NewPlanRes(plan models.EventPlan) PlanRes {
	return PlanRes{
		Code:           plan.Code,
		Name:           plan.Name,
		Description:    plan.Description,
		Price:          plan.Price,
		Currency:       plan.Currency,
		PhotoQuota:     plan.PhotoQuota,
		BoomerangQuota: plan.BoomerangQuota,
		RetentionDays:  plan.RetentionDays,
	}
}
