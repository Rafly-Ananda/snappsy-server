package drive

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	driveDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/drive"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type DriveHandler struct {
	drive *services.DriveService
}

func NewDriveHandler(drive *services.DriveService) *DriveHandler {
	return &DriveHandler{drive: drive}
}

func (h *DriveHandler) StartOperatorOAuth(c *gin.Context) {
	url, err := h.drive.OAuthStart(c.Request.Context(), "operator", "", c.Query("eventId"), "")
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"authUrl": url})
}

func (h *DriveHandler) OAuthCallback(c *gin.Context) {
	redirect, err := h.drive.OAuthCallback(c.Request.Context(), c.Query("state"), c.Query("code"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.Redirect(http.StatusFound, redirect)
}

func (h *DriveHandler) OperatorSession(c *gin.Context) {
	token, pickerKey, pickerAppID, err := h.drive.OperatorSession(c.Request.Context())
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, driveDto.SessionRes{AccessToken: token, PickerKey: pickerKey, PickerAppID: pickerAppID})
}

func (h *DriveHandler) SetOperatorDestination(c *gin.Context) {
	var req driveDto.DestinationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	destination, err := h.drive.SetOperatorDestination(c.Request.Context(), c.Param("id"), req.FolderID)
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, destination)
}

func (h *DriveHandler) CreateSetupLink(c *gin.Context) {
	url, expires, err := h.drive.CreateSetupLink(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, driveDto.SetupLinkRes{URL: url, ExpiresAt: expires.UTC().Format(time.RFC3339)})
}

func (h *DriveHandler) AdminStatus(c *gin.Context) {
	status, err := h.drive.AdminStatus(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *DriveHandler) Retry(c *gin.Context) {
	count, err := h.drive.Retry(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"retried": count})
}

func (h *DriveHandler) Reexport(c *gin.Context) {
	count, err := h.drive.ReexportEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"reset": count})
}

func (h *DriveHandler) SetupInfo(c *gin.Context) {
	event, connected, err := h.drive.SetupInfo(c.Request.Context(), c.Param("token"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	pickerKey, pickerAppID := h.drive.PickerConfig()
	c.JSON(http.StatusOK, driveDto.SetupInfoRes{
		EventID: event.ID.Hex(), EventName: event.Name, EventSlug: event.Slug, Connected: connected, PickerReady: pickerKey != "" && pickerAppID != "",
	})
}

func (h *DriveHandler) StartSetupOAuth(c *gin.Context) {
	event, _, err := h.drive.SetupInfo(c.Request.Context(), c.Param("token"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	url, err := h.drive.OAuthStart(c.Request.Context(), "customer", "", event.ID.Hex(), c.Param("token"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	c.Redirect(http.StatusFound, url)
}

func (h *DriveHandler) SetupSession(c *gin.Context) {
	token, pickerKey, err := h.drive.SetupSession(c.Request.Context(), c.Param("token"))
	if err != nil {
		writeDriveError(c, err)
		return
	}
	pickerAppID := ""
	if _, appID := h.drive.PickerConfig(); appID != "" {
		pickerAppID = appID
	}
	c.JSON(http.StatusOK, driveDto.SessionRes{AccessToken: token, PickerKey: pickerKey, PickerAppID: pickerAppID})
}

func (h *DriveHandler) SetCustomerDestination(c *gin.Context) {
	var req driveDto.DestinationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.drive.SetCustomerDestination(c.Request.Context(), c.Param("token"), req.FolderID); err != nil {
		writeDriveError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"connected": true})
}

func writeDriveError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrDriveNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "google drive is not configured"})
	case errors.Is(err, services.ErrDriveSetupTokenInvalid):
		c.JSON(http.StatusNotFound, gin.H{"error": "drive setup link is invalid or expired"})
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	}
}
