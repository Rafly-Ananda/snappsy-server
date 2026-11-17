package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	assetDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/assets"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type AssetHandler struct {
	service *services.AssetService
}

func NewAssetHandler(service *services.AssetService) *AssetHandler {
	return &AssetHandler{service: service}
}

func (h *AssetHandler) GeneratePresignedUpload(c *gin.Context) {
	var req assetDto.PresignAssetUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.GeneratePresignedUpload(c.Request.Context(), req)
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AssetHandler) CreateAsset(c *gin.Context) {
	var req assetDto.CreateAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.CreateAsset(c.Request.Context(), req)
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *AssetHandler) SetAssetDisabled(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Disabled *bool  `json:"disabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.UpdateAsset(c.Request.Context(), c.Param("id"), req.Name, req.Disabled)
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AssetHandler) ListEventAssets(c *gin.Context) {
	res, err := h.service.ListEventAssets(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

func (h *AssetHandler) ListAssets(c *gin.Context) {
	res, err := h.service.ListAssets(c.Request.Context(), c.Query("kind"))
	if err != nil {
		writeAssetError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

func writeAssetError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrInvalidAssetKind):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid asset kind"})
	case errors.Is(err, services.ErrUnsupportedMediaType):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrInvalidAssetKey), errors.Is(err, services.ErrUploadedObjectNotFound):
		c.JSON(http.StatusBadRequest, gin.H{"error": "uploaded asset was not found"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process asset"})
	}
}
