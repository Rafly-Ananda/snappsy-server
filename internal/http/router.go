package http

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/events"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/http/handlers/images"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/websocket"
)

type Handlers struct {
	Images *images.ImageHandler
	Events *events.EventHandler
	Websocket *websocket.WebSocketHandler
}

func NewRouter(h Handlers) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	// TODO Add CORS later here
	r.Use(cors.Default())

	// Websocket Setup
	hub := websocket.NewHub()
	go hub.Run()

	r.GET("/ws", h.Websocket.Handle)

	// HTTP Setup
	v1 := r.Group("/api/v1")
	{
		// Images
		img := v1.Group("/images")

		img.POST("/generate-uploader-url", h.Images.GeneratePresignedUploader)
		img.POST("", h.Images.CommitImageUpload)
		img.GET("/generate-url", h.Images.GeneratePresignedViewer)
		img.GET("/:eventId/slideshow-items", h.Images.GetAllImagesByEvent)
	}

	{
		events := v1.Group("/events")
		events.POST("/register", h.Events.RegisterEvent)
	}

	// simple health check
	r.GET("/health-check", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
		})
	})

	return r
}
