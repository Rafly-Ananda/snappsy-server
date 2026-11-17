package events

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type PublicEventHandler struct {
	events *services.EventService
	images *services.ImageService
}

func NewPublicEventHandler(events *services.EventService, images *services.ImageService) *PublicEventHandler {
	return &PublicEventHandler{events: events, images: images}
}

func (h *PublicEventHandler) GetBySlug(c *gin.Context) {
	res, err := h.events.GetPublicEventBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		writePublicEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PublicEventHandler) GetDisplaySlideshow(c *gin.Context) {
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if err != nil || limit < 1 || limit > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be between 1 and 50"})
		return
	}

	eventRes, event, err := h.events.GetEventByDisplayToken(c.Request.Context(), c.Param("token"))
	if err != nil {
		writePublicEventError(c, err)
		return
	}
	data, next, err := h.images.GetAllPresignedImagesByEvent(c.Request.Context(), event.ID.Hex(), c.Query("cursor"), limit, 10*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load display items"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"event":      eventRes,
		"data":       data,
		"nextCursor": next,
	})
}

func writePublicEventError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrEventNotFound) || errors.Is(err, services.ErrEventNotLive) {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load event"})
}
