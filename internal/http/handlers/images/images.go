package images

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/dto/images"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type ImageHandler struct {
	service *services.ImageService
}

func NewImageHandler(service *services.ImageService) *ImageHandler {
	return &ImageHandler{service: service}
}

// POST Requexsts
func (h *ImageHandler) CommitImageUpload(c *gin.Context) {
	var req images.CommitUploadReq

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.CommitImageUpload(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save image"})
		return
	}

	c.JSON(http.StatusOK, res)
}

func (h *ImageHandler) GeneratePresignedUploader(c *gin.Context) {
	var req images.GeneratePresignedUrlReq

	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := h.service.GeneratePresignedUploader(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate presigned url upload"})
		return
	}

	c.JSON(http.StatusOK, res)
}

// GET Requests
func (h *ImageHandler) GetAllImagesByEvent(c *gin.Context) {
	cursor := c.Query("cursor")
	eventId := c.Param("eventId")

	// TODO: make limit default query in constant
	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)

	// TODO: invalidated in 10 mins, need to be in env later
	data, next, err := h.service.GetAllPresignedImagesByEvent(c, eventId, cursor, limit, 10*time.Minute)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       data,
		"nextCursor": next, // "" means no more
	})
}

func (h *ImageHandler) GeneratePresignedViewer(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "object key"})
		return
	}

	// TODO: invalidated in 10 mins, need to be in env later
	res, err := h.service.GeneratePresignedViewer(c, key, 10*time.Minute)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
