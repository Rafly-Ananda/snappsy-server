package admin

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	templateDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/templates"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type TemplateHandler struct {
	service  *services.TemplateService
	eventSvc *services.EventService
}

func NewTemplateHandler(service *services.TemplateService, eventSvc *services.EventService) *TemplateHandler {
	return &TemplateHandler{service: service, eventSvc: eventSvc}
}

func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	templateID := c.Param("id")
	if err := h.eventSvc.ClearTemplateReference(c.Request.Context(), templateID); err != nil {
		writeTemplateError(c, err)
		return
	}
	if err := h.service.DeleteTemplate(c.Request.Context(), templateID); err != nil {
		writeTemplateError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	var req templateDto.CreateTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.CreateTemplate(c.Request.Context(), req.Name, req.Kind)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	res, err := h.service.ListTemplates(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list templates"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	res, err := h.service.GetTemplate(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *TemplateHandler) SaveDraft(c *gin.Context) {
	var req templateDto.SaveTemplateDraftReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.service.SaveDraft(c.Request.Context(), c.Param("id"), req.Schema)
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *TemplateHandler) PublishTemplate(c *gin.Context) {
	res, err := h.service.Publish(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeTemplateError(c, err)
		return
	}
	if err := h.eventSvc.UpdateTemplateVersion(c.Request.Context(), res.ID, res.LatestPublishedVer); err != nil {
		writeTemplateError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func writeTemplateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrTemplateNotFound), errors.Is(err, services.ErrTemplateVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "template not found"})
	case errors.Is(err, services.ErrInvalidTemplateSchema):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process template"})
	}
}
