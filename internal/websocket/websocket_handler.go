package websocket

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type WebSocketHandler struct {
	hub *Hub
}

func NewWebSocketHandler(hub *Hub) *WebSocketHandler {
	return &WebSocketHandler{hub: hub}
}

/*
 In produciton wss:// instead of ws:// to encrypt the communication channel with SSL/TLS
*/

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
       // TODO Implement your origin check logic here (Websocket Hijacking protection)
       // origin := r.Header.Get("Origin")
       // return origin == "<http://yourdomain.com>"
		return true
	},
}

func (h *WebSocketHandler) Handle(c *gin.Context) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	h.hub.register <- conn

	go func() {
		defer func() { h.hub.unregister <- conn }()
		for {
			// You can just ignore messages — clients only listen
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
