package templates

import (
	"time"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
)

type CreateTemplateReq struct {
	Name string `json:"name" binding:"required"`
	Kind string `json:"kind"`
}

type CreateEventTemplateReq struct {
	Name   string `json:"name" binding:"required"`
	Kind   string `json:"kind" binding:"required"`
	Preset string `json:"preset"`
	Paper  string `json:"paper"`
	Ink    string `json:"ink"`
	Accent string `json:"accent"`
	Title  string `json:"title"`
}

type UpdateEventTemplateStyleReq struct {
	Name  string               `json:"name" binding:"required"`
	Style models.TemplateStyle `json:"style" binding:"required"`
}

type SaveTemplateDraftReq struct {
	Schema models.TemplateSchema `json:"schema" binding:"required"`
}

type TemplateRes struct {
	ID                  string               `json:"id"`
	Kind                string               `json:"kind"`
	EventID             string               `json:"eventId,omitempty"`
	Name                string               `json:"name"`
	Style               models.TemplateStyle `json:"style,omitempty"`
	CurrentDraftVersion int                  `json:"currentDraftVersion"`
	LatestPublishedVer  int                  `json:"latestPublishedVersion"`
	CreatedAt           time.Time            `json:"createdAt"`
	UpdatedAt           time.Time            `json:"updatedAt"`
}

type TemplateVersionRes struct {
	ID         string                `json:"id"`
	TemplateID string                `json:"templateId"`
	Version    int                   `json:"version"`
	Status     string                `json:"status"`
	Schema     models.TemplateSchema `json:"schema"`
	CreatedAt  time.Time             `json:"createdAt"`
	UpdatedAt  time.Time             `json:"updatedAt"`
}

type TemplateDetailRes struct {
	TemplateRes
	Draft    TemplateVersionRes   `json:"draft"`
	Versions []TemplateVersionRes `json:"versions"`
}

func NewTemplateRes(template models.Template) TemplateRes {
	return TemplateRes{
		ID:                  template.ID.Hex(),
		Kind:                string(template.Kind),
		EventID:             template.EventID,
		Name:                template.Name,
		CurrentDraftVersion: template.CurrentDraftVer,
		LatestPublishedVer:  template.LatestPublishedVer,
		CreatedAt:           template.CreatedAt,
		UpdatedAt:           template.UpdatedAt,
	}
}

func NewTemplateVersionRes(version models.TemplateVersion) TemplateVersionRes {
	return TemplateVersionRes{
		ID:         version.ID.Hex(),
		TemplateID: version.TemplateID,
		Version:    version.Version,
		Status:     version.Status,
		Schema:     version.Schema,
		CreatedAt:  version.CreatedAt,
		UpdatedAt:  version.UpdatedAt,
	}
}
