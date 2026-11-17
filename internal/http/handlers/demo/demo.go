package demo

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type Handler struct {
	convert   *services.DemoConvertService
	settings  *services.SettingsService
	assets    *services.AssetService
	templates *services.TemplateService
}

func NewHandler(convert *services.DemoConvertService, settings *services.SettingsService, assets *services.AssetService, templates *services.TemplateService) *Handler {
	return &Handler{convert: convert, settings: settings, assets: assets, templates: templates}
}

func (h *Handler) demoTemplate(ctx context.Context, templateID string) any {
	if templateID == "" || h.templates == nil {
		return nil
	}
	schema, err := h.templates.PublishedSchema(ctx, templateID)
	if err != nil {
		return nil
	}
	assets := []any{}
	for _, layer := range schema.Layers {
		if layer.Type != models.TemplateLayerImage || layer.AssetID == "" {
			continue
		}
		asset, err := h.assets.PresignAsset(ctx, layer.AssetID)
		if err != nil {
			continue
		}
		assets = append(assets, asset)
	}
	return gin.H{"id": templateID, "schema": schema, "assets": assets}
}

func (h *Handler) Settings(c *gin.Context) {
	demo, err := h.settings.Demo(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "settings lookup failed"})
		return
	}
	stickers := []any{}
	if assets, err := h.assets.ListPlatformAssets(c.Request.Context(), "sticker"); err == nil {
		allowed := make(map[string]bool, len(demo.StickerAssetIDs))
		for _, id := range demo.StickerAssetIDs {
			allowed[id] = true
		}
		for _, asset := range assets {
			if asset.Disabled || (len(allowed) > 0 && !allowed[asset.ID]) {
				continue
			}
			stickers = append(stickers, asset)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"enabled":         demo.Enabled,
		"photo":           demo.Photo,
		"strip":           demo.Strip,
		"boomerang":       demo.Boomerang,
		"stickerAssetIds": demo.StickerAssetIDs,
		"countdowns":      demo.Countdowns.Normalized(),
		"stickers":        stickers,
		"templates": gin.H{
			"photo":     h.demoTemplate(c.Request.Context(), demo.PhotoTemplateID),
			"strip":     h.demoTemplate(c.Request.Context(), demo.StripTemplateID),
			"boomerang": h.demoTemplate(c.Request.Context(), demo.BoomerangTemplateID),
		},
	})
}

func (h *Handler) ConvertBoomerang(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 31<<20)
	fileHeader, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose a video file"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > 30<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "choose an MP4, WebM or MOV clip under 30 MB"})
		return
	}
	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the clip could not be read"})
		return
	}
	defer file.Close()
	input, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "the clip could not be read"})
		return
	}

	output, err := h.convert.Convert(c.Request.Context(), input, extension)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrDemoConvertBusy):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrDemoConvertUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrDemoConvertInput):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": services.ErrDemoConvertOutput.Error()})
		}
		return
	}
	c.Header("Content-Disposition", `attachment; filename="snappsy-boomerang.mp4"`)
	c.Data(http.StatusOK, "video/mp4", output)
}
