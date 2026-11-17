package websocket

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/identity"
	"github.com/rafly-ananda/snappsy-uploader-api/internal/repositories"
)

type WebSocketHandler struct {
	hub            *Hub
	allowedOrigins []string
	events         repositories.EventRepository
}

func NewDisplayWebSocketHandler(hub *Hub, events repositories.EventRepository, allowedOrigins []string) *WebSocketHandler {
	return &WebSocketHandler{hub: hub, events: events, allowedOrigins: allowedOrigins}
}

func (h *WebSocketHandler) HandleDisplay(c *gin.Context) {
	event, err := h.events.FindByDisplayTokenHash(c.Request.Context(), identity.HashToken(c.Param("token")))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "display link not found"})
		return
	}
	h.serve(c, event.ID.Hex())
}

func (h *WebSocketHandler) serve(c *gin.Context, eventID string) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return isAllowedOrigin(r.Header.Get("Origin"), h.allowedOrigins)
		},
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.hub.registerClient(conn, eventID)
}

func isAllowedOrigin(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	for _, allowed := range allowedOrigins {
		allowedURL, err := url.Parse(allowed)
		if err != nil || allowedURL.Host == "" {
			continue
		}
		if strings.EqualFold(u.Scheme, allowedURL.Scheme) && strings.EqualFold(u.Host, allowedURL.Host) {
			return true
		}
	}
	return false
}
