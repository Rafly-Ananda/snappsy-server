package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 5 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
)

type Client struct {
	conn      *websocket.Conn
	eventID   string
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
}

type publication struct {
	eventID string
	payload []byte
}

type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	publish    chan publication
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		publish:    make(chan publication, 64),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.close()
			}
		case message := <-h.publish:
			for client := range h.clients {
				if client.eventID != message.eventID {
					continue
				}
				select {
				case client.send <- message.payload:
				default:
					log.Printf("disconnecting slow websocket client event=%s", client.eventID)
					delete(h.clients, client)
					client.close()
				}
			}
		}
	}
}

// Publish queues a hint for one event. REST remains authoritative for recovery.
func (h *Hub) Publish(eventID, message string) {
	select {
	case h.publish <- publication{eventID: eventID, payload: []byte(message)}:
	default:
		log.Println("websocket publish queue full; dropping hint")
	}
}

func MediaReadyMessage(eventID, mediaID, kind string) string {
	payload, _ := json.Marshal(map[string]any{
		"version": 1,
		"type":    "media.ready",
		"eventId": eventID,
		"mediaId": mediaID,
		"kind":    kind,
	})
	return string(payload)
}

func MediaChangedMessage(eventID, mediaID, kind string) string {
	payload, _ := json.Marshal(map[string]any{
		"version": 1,
		"type":    "media.changed",
		"eventId": eventID,
		"mediaId": mediaID,
		"kind":    kind,
	})
	return string(payload)
}

func (h *Hub) registerClient(conn *websocket.Conn, eventID string) *Client {
	client := &Client{conn: conn, eventID: eventID, send: make(chan []byte, 16), done: make(chan struct{})}
	h.register <- client
	go h.writePump(client)
	go h.readPump(client)
	return client
}

func (h *Hub) readPump(client *Client) {
	defer func() { h.unregister <- client }()
	client.conn.SetReadLimit(1024)
	_ = client.conn.SetReadDeadline(time.Now().Add(pongWait))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := client.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-client.done:
			return
		case message := <-client.send:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				client.close()
				return
			}
		case <-ticker.C:
			_ = client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				client.close()
				return
			}
		}
	}
}

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
