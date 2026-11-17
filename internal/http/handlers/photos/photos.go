package photos

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	photoDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type PhotoHandler struct {
	service *services.PhotoService
}

func NewPhotoHandler(service *services.PhotoService) *PhotoHandler {
	return &PhotoHandler{service: service}
}

func (h *PhotoHandler) CreateIntent(c *gin.Context) {
	var req photoDto.CreateIntentReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.CreateIntent(c.Request.Context(), req)
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *PhotoHandler) Commit(c *gin.Context) {
	var req photoDto.CommitReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.Commit(c.Request.Context(), req)
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *PhotoHandler) Status(c *gin.Context) {
	res, err := h.service.Status(c.Request.Context(), c.Param("id"), c.Query("token"))
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PhotoHandler) Download(c *gin.Context) {
	url, err := h.service.Download(c.Request.Context(), c.Param("id"), c.Query("token"))
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"url": url})
}

func (h *PhotoHandler) Design(c *gin.Context) {
	res, err := h.service.PublicDesign(c.Request.Context(), c.Param("slug"))
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PhotoHandler) ListMedia(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.service.ListForEvent(c.Request.Context(), c.Param("id"), c.Query("cursor"), limit, c.Query("state"))
	if err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *PhotoHandler) Moderate(c *gin.Context) {
	var req photoDto.ModerationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.SetModerationState(c.Request.Context(), c.Param("id"), req.State); err != nil {
		writePhotoError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "moderationState": req.State})
}

func writePhotoError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrEventNotFound), errors.Is(err, services.ErrPhotoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	case errors.Is(err, services.ErrEventNotAcceptingUploads), errors.Is(err, services.ErrEventQuotaExceeded):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrCaptureModeDisabled):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrPhotoIntentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "upload intent not found"})
	case errors.Is(err, services.ErrPhotoIntentExpired):
		c.JSON(http.StatusGone, gin.H{"error": "upload intent expired"})
	case errors.Is(err, services.ErrPhotoStatusTokenInvalid):
		c.JSON(http.StatusForbidden, gin.H{"error": "status token is invalid"})
	case errors.Is(err, services.ErrPhotoDownloadNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"error": "downloads are not enabled for this event"})
	case errors.Is(err, services.ErrPhotoNotReady):
		c.JSON(http.StatusConflict, gin.H{"error": "media is not ready"})
	case errors.Is(err, services.ErrPhotoConsentRequired), errors.Is(err, services.ErrInvalidModerationState):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrUnsupportedMediaType), errors.Is(err, services.ErrUploadedObjectNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "photo request failed"})
	}
}
