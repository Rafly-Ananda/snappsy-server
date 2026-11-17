package boomerangs

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	boomerangDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/boomerangs"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type BoomerangHandler struct {
	service *services.BoomerangService
}

func NewBoomerangHandler(service *services.BoomerangService) *BoomerangHandler {
	return &BoomerangHandler{service: service}
}

func (h *BoomerangHandler) CreateIntent(c *gin.Context) {
	var req boomerangDto.CreateIntentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.CreateIntent(c.Request.Context(), req)
	if err != nil {
		writeBoomerangError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *BoomerangHandler) Commit(c *gin.Context) {
	var req boomerangDto.CommitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.Commit(c.Request.Context(), req)
	if err != nil {
		writeBoomerangError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *BoomerangHandler) Status(c *gin.Context) {
	res, err := h.service.Status(c.Request.Context(), c.Param("id"), c.Query("token"))
	if err != nil {
		writeBoomerangError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *BoomerangHandler) QueueStatus(c *gin.Context) {
	res, err := h.service.QueueStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeBoomerangError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *BoomerangHandler) Download(c *gin.Context) {
	url, err := h.service.Download(c.Request.Context(), c.Param("id"), c.Query("token"))
	if err != nil {
		writeBoomerangError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func writeBoomerangError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrEventNotFound), errors.Is(err, services.ErrPhotoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, services.ErrEventNotAcceptingUploads), errors.Is(err, services.ErrEventQuotaExceeded), errors.Is(err, services.ErrBoomerangQueueFull):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrCaptureModeDisabled):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPhotoIntentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "upload intent not found"})
	case errors.Is(err, services.ErrPhotoIntentExpired):
		c.JSON(http.StatusGone, gin.H{"error": "upload intent expired"})
	case errors.Is(err, services.ErrBoomerangStatusToken), errors.Is(err, services.ErrPhotoStatusTokenInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": "status token is invalid"})
	case errors.Is(err, services.ErrPhotoDownloadNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "downloads are not enabled for this event"})
	case errors.Is(err, services.ErrPhotoNotReady):
		c.JSON(http.StatusConflict, gin.H{"error": "media is not ready"})
	case errors.Is(err, services.ErrPhotoConsentRequired),
		errors.Is(err, services.ErrBoomerangInvalidSegment),
		errors.Is(err, services.ErrBoomerangInvalidSource),
		errors.Is(err, services.ErrBoomerangSourceTooLarge),
		errors.Is(err, services.ErrUploadedObjectNotFound),
		errors.Is(err, services.ErrUnsupportedMediaType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		log.Printf("boomerang request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "boomerang request failed"})
	}
}
