package http

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	adminHandler "github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/admin"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/boomerangs"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/customer"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/demo"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/drive"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/photos"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
)

type Handlers struct {
	Photos           *photos.PhotoHandler
	Boomerangs       *boomerangs.BoomerangHandler
	PublicEvents     *events.PublicEventHandler
	Admin            *adminHandler.AdminHandler
	AdminAssets      *adminHandler.AssetHandler
	AdminTemplates   *adminHandler.TemplateHandler
	Drive            *drive.DriveHandler
	DisplayWebsocket *websocket.WebSocketHandler
	AdminAuth        gin.HandlerFunc
	AdminCSRF        gin.HandlerFunc
	Customer         *customer.Handler
	Demo             *demo.Handler
	AdminSettings    *adminHandler.SettingsHandler
	AdminPlans       *adminHandler.PlanHandler
	CustomerAuth     gin.HandlerFunc
	CustomerCSRF     gin.HandlerFunc
}

func NewRouter(h Handlers, allowedOrigins []string) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     allowedOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	v1 := r.Group("/api/v1")
	{
		v1.GET("/demo/settings", h.Demo.Settings)
		v1.POST("/demo/boomerang-convert", h.Demo.ConvertBoomerang)
		v1.POST("/auth/signup", h.Customer.Signup)
		v1.POST("/auth/login", h.Customer.Login)
		v1.GET("/plans", h.Customer.ListPlans)
		v1.GET("/events/slug/:slug", h.PublicEvents.GetBySlug)
		v1.GET("/events/slug/:slug/design", h.Photos.Design)
		v1.GET("/display/:token/slideshow-items", h.PublicEvents.GetDisplaySlideshow)
		v1.GET("/display/:token/ws", h.DisplayWebsocket.HandleDisplay)
		v1.GET("/drive/oauth/callback", h.Drive.OAuthCallback)
		v1.GET("/drive/setup/:token", h.Drive.SetupInfo)
		v1.GET("/drive/setup/:token/oauth/start", h.Drive.StartSetupOAuth)
		v1.GET("/drive/setup/:token/session", h.Drive.SetupSession)
		v1.POST("/drive/setup/:token/destination", h.Drive.SetCustomerDestination)

		photoRoutes := v1.Group("/photos")
		photoRoutes.POST("/intents", h.Photos.CreateIntent)
		photoRoutes.POST("/commit", h.Photos.Commit)
		photoRoutes.GET("/:id/status", h.Photos.Status)
		photoRoutes.GET("/:id/download", h.Photos.Download)

		boomerangRoutes := v1.Group("/boomerangs")
		boomerangRoutes.POST("/intents", h.Boomerangs.CreateIntent)
		boomerangRoutes.POST("/commit", h.Boomerangs.Commit)
		boomerangRoutes.GET("/:id/status", h.Boomerangs.Status)
		boomerangRoutes.GET("/:id/download", h.Boomerangs.Download)

		v1.POST("/auth/logout", h.CustomerAuth, h.CustomerCSRF, h.Customer.Logout)
		v1.GET("/auth/me", h.CustomerAuth, h.Customer.Me)

		customerAuthed := v1.Group("/me", h.CustomerAuth, h.CustomerCSRF)
		customerAuthed.GET("/events", h.Customer.ListEvents)
		customerAuthed.POST("/events", h.Customer.CreateEvent)
		customerAuthed.GET("/events/:id", h.Customer.GetEvent)
		customerAuthed.PATCH("/events/:id", h.Customer.UpdateEvent)
		customerAuthed.POST("/events/:id/publish", h.Customer.PublishEvent)
		customerAuthed.POST("/events/:id/archive", h.Customer.ArchiveEvent)
		customerAuthed.POST("/events/:id/display-token", h.Customer.RotateDisplayToken)
		customerAuthed.GET("/events/:id/media", h.Customer.ListEventMedia)
		customerAuthed.GET("/events/:id/assets", h.Customer.ListEventAssets)
		customerAuthed.POST("/events/:id/assets/presign", h.Customer.PresignEventAsset)
		customerAuthed.POST("/events/:id/assets", h.Customer.CreateEventAsset)
		customerAuthed.PATCH("/events/:id/assets/:assetId", h.Customer.UpdateEventAsset)
		customerAuthed.GET("/events/:id/templates", h.Customer.ListEventTemplates)
		customerAuthed.POST("/events/:id/templates", h.Customer.CreateEventTemplate)
		customerAuthed.DELETE("/events/:id/design/:kind", h.Customer.ResetEventDesign)
		customerAuthed.PUT("/events/:id/templates/:templateId", h.Customer.UpdateEventTemplate)
		customerAuthed.DELETE("/events/:id/templates/:templateId", h.Customer.DeleteEventTemplate)
		customerAuthed.PATCH("/media/:id/moderation", h.Customer.ModerateMedia)
		customerAuthed.GET("/drive/status", h.Customer.DriveStatus)
		customerAuthed.GET("/drive/session", h.Customer.DriveSession)
		customerAuthed.POST("/drive/oauth/start", h.Customer.StartDriveOAuth)
		customerAuthed.POST("/drive/destination", h.Customer.SetDriveDestination)

		adminPublic := v1.Group("/admin")
		adminPublic.POST("/login", h.Admin.Login)

		admin := v1.Group("/admin", h.AdminAuth, h.AdminCSRF)
		admin.POST("/logout", h.Admin.Logout)
		admin.GET("/me", h.Admin.Me)
		admin.GET("/settings/demo", h.AdminSettings.GetDemo)
		admin.PATCH("/settings/demo", h.AdminSettings.UpdateDemo)
		admin.GET("/events", h.Admin.ListEvents)
		admin.POST("/events", h.Admin.CreateEvent)
		admin.GET("/events/:id", h.Admin.GetEvent)
		admin.PATCH("/events/:id", h.Admin.UpdateEvent)
		admin.DELETE("/events/:id/design/:kind", h.Admin.ResetEventDesign)
		admin.POST("/events/:id/publish", h.Admin.PublishEvent)
		admin.POST("/events/:id/archive", h.Admin.ArchiveEvent)
		admin.POST("/events/:id/display-token/rotate", h.Admin.RotateDisplayToken)
		admin.GET("/plans", h.AdminPlans.ListPlans)
		admin.PATCH("/plans/:code", h.AdminPlans.UpdatePlan)
		admin.PATCH("/events/:id/plan", h.AdminPlans.UpdateEventPlan)

		admin.POST("/assets/presign", h.AdminAssets.GeneratePresignedUpload)
		admin.POST("/assets", h.AdminAssets.CreateAsset)
		admin.GET("/assets", h.AdminAssets.ListAssets)
		admin.GET("/events/:id/assets", h.AdminAssets.ListEventAssets)
		admin.PATCH("/assets/:id", h.AdminAssets.SetAssetDisabled)

		admin.POST("/templates", h.AdminTemplates.CreateTemplate)
		admin.GET("/templates", h.AdminTemplates.ListTemplates)
		admin.GET("/templates/:id", h.AdminTemplates.GetTemplate)
		admin.PUT("/templates/:id/draft", h.AdminTemplates.SaveDraft)
		admin.POST("/templates/:id/publish", h.AdminTemplates.PublishTemplate)
		admin.DELETE("/templates/:id", h.AdminTemplates.DeleteTemplate)
		admin.GET("/events/:id/media", h.Photos.ListMedia)
		admin.GET("/events/:id/boomerang-queue", h.Boomerangs.QueueStatus)
		admin.PATCH("/media/:id/moderation", h.Photos.Moderate)
		admin.GET("/drive/oauth/start", h.Drive.StartOperatorOAuth)
		admin.GET("/drive/session", h.Drive.OperatorSession)
		admin.GET("/events/:id/drive", h.Drive.AdminStatus)
		admin.POST("/events/:id/drive/destination", h.Drive.SetOperatorDestination)
		admin.POST("/events/:id/drive/setup-link", h.Drive.CreateSetupLink)
		admin.POST("/events/:id/drive/retry", h.Drive.Retry)
		admin.POST("/events/:id/drive/reexport", h.Drive.Reexport)
	}

	r.GET("/health-check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
	})
	return r
}
