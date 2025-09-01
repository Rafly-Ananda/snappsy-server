package events

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type EventHandler struct {
	service *services.EventService
}

func NewEventHandler(service *services.EventService) *EventHandler {
	return &EventHandler{service: service}
}

// POST Requexsts
func (h *EventHandler) RegisterEvent(c *gin.Context) {
	var req events.CreateEventReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.RegisterEvent(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
		return
	}

	c.JSON(http.StatusOK, res)
}
