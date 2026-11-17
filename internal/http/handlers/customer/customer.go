package customer

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	assetDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/assets"
	customerDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/customer"
	eventDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/events"
	photoDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/photos"
	templateDto "github.com/rafly-ananda/snappsy-uploader-api/internal/dto/templates"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/middleware"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/models"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/services"
)

type Handler struct {
	customers *services.CustomerService
	plans     *services.PlanService
	events    *services.EventService
	photos    *services.PhotoService
	drive     *services.DriveService
	assets    *services.AssetService
	templates *services.TemplateService
}

func NewHandler(customers *services.CustomerService, plans *services.PlanService, events *services.EventService, photos *services.PhotoService, drive *services.DriveService, assets *services.AssetService, templates *services.TemplateService) *Handler {
	return &Handler{customers: customers, plans: plans, events: events, photos: photos, drive: drive, assets: assets, templates: templates}
}

func (h *Handler) DriveStatus(c *gin.Context) {
	status, err := h.drive.CustomerStatus(c.Request.Context(), currentCustomer(c).ID.Hex(), c.Query("eventId"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, status)
}

func (h *Handler) DriveSession(c *gin.Context) {
	token, pickerKey, pickerAppID, err := h.drive.CustomerSession(c.Request.Context(), currentCustomer(c).ID.Hex(), c.Query("eventId"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accessToken": token, "pickerApiKey": pickerKey, "pickerAppId": pickerAppID})
}

func (h *Handler) StartDriveOAuth(c *gin.Context) {
	eventID := c.Query("eventId")
	url, err := h.drive.StartCustomerOAuth(c.Request.Context(), currentCustomer(c).ID.Hex(), eventID)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"authUrl": url})
}

func (h *Handler) SetDriveDestination(c *gin.Context) {
	var req struct {
		EventID  string `json:"eventId" binding:"required"`
		FolderID string `json:"folderId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Event and Drive folder are required."})
		return
	}
	destination, err := h.drive.SetCustomerDestinationForOwner(c.Request.Context(), currentCustomer(c).ID.Hex(), req.EventID, req.FolderID)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, destination)
}

func (h *Handler) Signup(c *gin.Context) {
	var req customerDto.SignupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and a password of at least 12 characters are required."})
		return
	}
	customer, token, csrf, expiresAt, err := h.customers.Signup(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, services.ErrCustomerExists):
			c.JSON(http.StatusConflict, gin.H{"error": "An account with this email already exists."})
		case errors.Is(err, services.ErrCustomerInvalidCredentials):
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email and a password of at least 12 characters are required."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create the account."})
		}
		return
	}
	setCookies(c, token, csrf, expiresAt, h.customers.SecureCookies())
	c.JSON(http.StatusCreated, customerDto.AuthRes{User: customerDto.NewUserRes(customer)})
}

func (h *Handler) Login(c *gin.Context) {
	var req customerDto.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email and password are required."})
		return
	}
	customer, token, csrf, expiresAt, err := h.customers.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, services.ErrTooManyAttempts):
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many login attempts. Try again later."})
		case errors.Is(err, services.ErrCustomerInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password."})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not log in."})
		}
		return
	}
	setCookies(c, token, csrf, expiresAt, h.customers.SecureCookies())
	c.JSON(http.StatusOK, customerDto.AuthRes{User: customerDto.NewUserRes(customer)})
}

func (h *Handler) Logout(c *gin.Context) {
	if token, err := c.Cookie(middleware.CustomerSessionCookie); err == nil {
		_ = h.customers.Logout(c.Request.Context(), token)
	}
	clearCookies(c, h.customers.SecureCookies())
	c.JSON(http.StatusOK, gin.H{"authenticated": false})
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, customerDto.AuthRes{User: customerDto.NewUserRes(currentCustomer(c))})
}

func (h *Handler) ListPlans(c *gin.Context) {
	plans, err := h.plans.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load plans."})
		return
	}
	out := make([]customerDto.PlanRes, 0, len(plans))
	for _, plan := range plans {
		out = append(out, customerDto.NewPlanRes(plan))
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

func (h *Handler) ListEvents(c *gin.Context) {
	events, err := h.events.ListCustomerEvents(c.Request.Context(), currentCustomer(c).ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load events."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": events})
}

func (h *Handler) CreateEvent(c *gin.Context) {
	var req eventDto.CreateCustomerEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, plan, email and WhatsApp number are required."})
		return
	}
	plan, err := h.plans.Find(c.Request.Context(), req.PlanCode)
	if err != nil {
		if errors.Is(err, services.ErrPlanNotFound) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "That event plan is not available."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load the event plan."})
		return
	}
	res, err := h.events.CreateCustomerEvent(c.Request.Context(), currentCustomer(c).ID.Hex(), req, plan)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not create the event."})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) GetEvent(c *gin.Context) {
	res, err := h.events.GetCustomerEvent(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex())
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) UpdateEvent(c *gin.Context) {
	var req eventDto.UpdateEventReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TemplateID != nil && strings.TrimSpace(*req.TemplateID) != "" {
		if err := h.templates.ValidateEventAssignment(c.Request.Context(), c.Param("id"), *req.TemplateID, models.TemplateKindPhoto); err != nil {
			writeEventError(c, err)
			return
		}
	}
	if req.StripTemplateID != nil && strings.TrimSpace(*req.StripTemplateID) != "" {
		if err := h.templates.ValidateEventAssignment(c.Request.Context(), c.Param("id"), *req.StripTemplateID, models.TemplateKindStrip); err != nil {
			writeEventError(c, err)
			return
		}
	}
	if req.BoomerangTemplateID != nil && strings.TrimSpace(*req.BoomerangTemplateID) != "" {
		if err := h.templates.ValidateEventAssignment(c.Request.Context(), c.Param("id"), *req.BoomerangTemplateID, models.TemplateKindVideoFrame); err != nil {
			writeEventError(c, err)
			return
		}
	}
	res, err := h.events.UpdateCustomerEvent(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex(), req)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ResetEventDesign(c *gin.Context) {
	res, err := h.events.ResetCustomerTemplate(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex(), c.Param("kind"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) PublishEvent(c *gin.Context) {
	res, err := h.events.PublishCustomerEvent(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex())
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ArchiveEvent(c *gin.Context) {
	res, err := h.events.ArchiveCustomerEvent(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex())
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) RotateDisplayToken(c *gin.Context) {
	res, err := h.events.RotateCustomerDisplayToken(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex())
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListEventMedia(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))
	res, err := h.photos.ListForCustomerEvent(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex(), c.Query("cursor"), limit, c.Query("state"))
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ModerateMedia(c *gin.Context) {
	var req photoDto.ModerationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.photos.SetModerationStateForOwner(c.Request.Context(), c.Param("id"), currentCustomer(c).ID.Hex(), req.State); err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "moderationState": req.State})
}

func currentCustomer(c *gin.Context) models.Customer {
	customer, _ := c.Get("customer")
	return customer.(models.Customer)
}

func (h *Handler) ListEventAssets(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	platform, err := h.assets.ListPlatformAssets(c.Request.Context(), string(models.AssetKindSticker))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load stickers."})
		return
	}
	eventAssets, err := h.assets.ListEventAssets(c.Request.Context(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load event stickers."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"platform": platform, "event": eventAssets})
}

func (h *Handler) PresignEventAsset(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	var req assetDto.PresignAssetUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.assets.GenerateEventPresignedUpload(c.Request.Context(), eventID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CreateEventAsset(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	var req assetDto.CreateAssetReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.assets.CreateEventAsset(c.Request.Context(), eventID, currentCustomer(c).ID.Hex(), req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) UpdateEventAsset(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A sticker name is required."})
		return
	}
	res, err := h.assets.UpdateEventAsset(c.Request.Context(), eventID, c.Param("assetId"), req.Name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Sticker not found."})
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListEventTemplates(c *gin.Context) {
	eventID := c.Param("id")
	event, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex())
	if err != nil {
		writeEventError(c, err)
		return
	}
	own, err := h.templates.ListEventTemplates(c.Request.Context(), eventID, c.Query("kind"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load templates."})
		return
	}
	platform, err := h.templates.ListPlatformTemplates(c.Request.Context(), event.AllowedTemplateIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load templates."})
		return
	}
	kind := c.Query("kind")
	res := append(platform, own...)
	if kind != "" {
		filtered := res[:0]
		for _, item := range res {
			if item.Kind == kind || (item.Kind == "" && kind == "photo") {
				filtered = append(filtered, item)
			}
		}
		res = filtered
	}
	c.JSON(http.StatusOK, gin.H{"data": res})
}

func (h *Handler) DeleteEventTemplate(c *gin.Context) {
	templateID := c.Param("templateId")
	if err := h.templates.DeleteEventTemplate(c.Request.Context(), currentCustomer(c).ID.Hex(), c.Param("id"), templateID); err != nil {
		writeEventError(c, err)
		return
	}
	if err := h.events.ClearTemplateReference(c.Request.Context(), templateID); err != nil {
		writeEventError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateEventTemplate(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	var req templateDto.CreateEventTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.templates.CreateEventTemplate(c.Request.Context(), currentCustomer(c).ID.Hex(), eventID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) UpdateEventTemplate(c *gin.Context) {
	eventID := c.Param("id")
	if _, err := h.events.GetCustomerEvent(c.Request.Context(), eventID, currentCustomer(c).ID.Hex()); err != nil {
		writeEventError(c, err)
		return
	}
	var req templateDto.UpdateEventTemplateStyleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	res, err := h.templates.UpdateEventTemplateStyle(c.Request.Context(), currentCustomer(c).ID.Hex(), eventID, c.Param("templateId"), req)
	if err != nil {
		writeEventError(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

func writeEventError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, services.ErrEventNotFound), errors.Is(err, services.ErrTemplateNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Event or template not found."})
	case errors.Is(err, services.ErrTemplateAssignment), errors.Is(err, services.ErrInvalidTemplateSchema):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrEventNotApproved):
		c.JSON(http.StatusForbidden, gin.H{"error": "This event plan is not active yet."})
	case errors.Is(err, services.ErrPhotoNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "Media not found."})
	case errors.Is(err, services.ErrInvalidModerationState):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrTemplateAssignment):
		c.JSON(http.StatusBadRequest, gin.H{"error": "That template version cannot be assigned."})
	case errors.Is(err, services.ErrDriveNotConfigured):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google Drive is not available right now."})
	case errors.Is(err, services.ErrDriveSetupTokenInvalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, services.ErrDriveFolderInaccessible):
		c.JSON(http.StatusBadRequest, gin.H{"error": services.ErrDriveFolderInaccessible.Error()})
	case errors.Is(err, services.ErrDriveReconnect):
		c.JSON(http.StatusConflict, gin.H{"error": services.ErrDriveReconnect.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Event request failed."})
	}
}

func setCookies(c *gin.Context, token, csrf string, expiresAt time.Time, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.CustomerSessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.CustomerCSRFCookie,
		Value:    csrf,
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
}

func clearCookies(c *gin.Context, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.CustomerSessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     middleware.CustomerCSRFCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: false,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
