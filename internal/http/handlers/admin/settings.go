package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type SettingsHandler struct {
	settings *services.SettingsService
}

func NewSettingsHandler(settings *services.SettingsService) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) GetDemo(c *gin.Context) {
	demo, err := h.settings.Demo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings lookup failed"})
		return
	}
	c.JSON(http.StatusOK, demo)
}

func (h *SettingsHandler) UpdateDemo(c *gin.Context) {
	var req models.DemoSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	demo, err := h.settings.UpdateDemo(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings update failed"})
		return
	}
	c.JSON(http.StatusOK, demo)
}
