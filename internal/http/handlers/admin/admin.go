package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	adminDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/admin"
	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/middleware"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type AdminHandler struct {
	adminSvc *services.AdminService
	eventSvc *services.EventService
}

func NewAdminHandler(adminSvc *services.AdminService, eventSvc *services.EventService) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc, eventSvc: eventSvc}
}

func (h *AdminHandler) Login(c *gin.Context) {
	var req adminDto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	token, csrf, expiresAt, err := h.adminSvc.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTooManyAttempts):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many login attempts"})
		case errors.Is(err, services.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to login"})
		}
		return
	}

	setAdminCookies(c, token, csrf, expiresAt, h.adminSvc.SecureCookies())
	c.JSON(http.StatusOK, adminDto.LoginRes{Authenticated: true, Username: h.adminSvc.Username()})
}

func (h *AdminHandler) Logout(c *gin.Context) {
	if token, err := c.Cookie(middleware.AdminSessionCookie); err == nil {
		_ = h.adminSvc.Logout(c.Request.Context(), token)
	}
	clearAdminCookies(c, h.adminSvc.SecureCookies())
	c.JSON(http.StatusOK, gin.H{"authenticated": false})
}

func (h *AdminHandler) Me(c *gin.Context) {
	username, _ := c.Get("adminUsername")
	c.JSON(http.StatusOK, adminDto.MeRes{Username: username.(string)})
}

func (h *AdminHandler) ListEvents(c *gin.Context) {
	events, err := h.eventSvc.ListEvents(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list events"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}

func (h *AdminHandler) CreateEvent(c *gin.Context) {
	var req eventDto.CreateEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.eventSvc.CreateEvent(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create event"})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *AdminHandler) GetEvent(c *gin.Context) {
	res, err := h.eventSvc.GetEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) ResetEventDesign(c *gin.Context) {
	res, err := h.eventSvc.ResetTemplate(c.Request.Context(), c.Param("id"), c.Param("kind"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) UpdateEvent(c *gin.Context) {
	var req eventDto.UpdateEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.eventSvc.UpdateEvent(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) PublishEvent(c *gin.Context) {
	res, err := h.eventSvc.PublishEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) ArchiveEvent(c *gin.Context) {
	res, err := h.eventSvc.ArchiveEvent(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *AdminHandler) RotateDisplayToken(c *gin.Context) {
	res, err := h.eventSvc.RotateDisplayToken(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func setAdminCookies(c *gin.Context, token, csrf string, expiresAt time.Time, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminSessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminCSRFCookie,
		Value:    csrf,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
}

func clearAdminCookies(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminSessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.AdminCSRFCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func writeEventError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrEventNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
	case errors.Is(err, services.ErrTemplateNotFound), errors.Is(err, services.ErrTemplateAssignment):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
	}
}
