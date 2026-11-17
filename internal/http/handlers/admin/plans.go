package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	customerDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/customer"
	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type PlanHandler struct {
	plans  *services.PlanService
	events *services.EventService
}

type updatePlanCatalogReq struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Price          int64  `json:"price"`
	Currency       string `json:"currency"`
	PhotoQuota     int    `json:"photoQuota"`
	BoomerangQuota int    `json:"boomerangQuota"`
	RetentionDays  int    `json:"retentionDays"`
}

func NewPlanHandler(plans *services.PlanService, events *services.EventService) *PlanHandler {
	return &PlanHandler{plans: plans, events: events}
}

func (h *PlanHandler) ListPlans(c *gin.Context) {
	plans, err := h.plans.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load plans."})
		return
	}
	out := make([]customerDto.PlanRes, 0, len(plans))
	for _, plan := range plans {
		out = append(out, customerDto.NewPlanRes(plan))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *PlanHandler) UpdatePlan(c *gin.Context) {
	var req updatePlanCatalogReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	plan, err := h.plans.Update(c.Request.Context(), models.EventPlan{
		Code:           c.Param("code"),
		Name:           req.Name,
		Description:    req.Description,
		Price:          req.Price,
		Currency:       req.Currency,
		PhotoQuota:     req.PhotoQuota,
		BoomerangQuota: req.BoomerangQuota,
		RetentionDays:  req.RetentionDays,
		Active:         true,
	})
	if err != nil {
		if errors.Is(err, services.ErrPlanNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Plan not found."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not update the plan."})
		return
	}
	c.JSON(http.StatusOK, customerDto.NewPlanRes(plan))
}

func (h *PlanHandler) UpdateEventPlan(c *gin.Context) {
	var req eventDto.UpdatePlanReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var plan *models.EventPlan
	if req.PlanCode != "" {
		found, err := h.plans.Find(c.Request.Context(), req.PlanCode)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "That event plan is not available."})
			return
		}
		plan = &found
	}
	res, err := h.events.UpdatePlan(c.Request.Context(), c.Param("id"), req, plan)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}
