package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPublishIsScopedToEvent(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.registerClient(conn, r.URL.Query().Get("event"))
	}))
	defer server.Close()

	dial := func(eventID string) *websocket.Conn {
		url := "ws" + strings.TrimPrefix(server.URL, "http") + "?event=" + eventID
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatalf("dial event %s: %v", eventID, err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	clientA := dial("event-a")
	clientB := dial("event-b")
	time.Sleep(20 * time.Millisecond)

	hub.Publish("event-a", MediaReadyMessage("event-a", "media-a", "photo"))
	_ = clientA.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, payload, err := clientA.ReadMessage()
	if err != nil {
		t.Fatalf("event A read: %v", err)
	}
	if !strings.Contains(string(payload), `"type":"media.ready"`) || !strings.Contains(string(payload), `"eventId":"event-a"`) {
		t.Fatalf("unexpected event payload: %s", payload)
	}

	_ = clientB.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if _, _, err := clientB.ReadMessage(); err == nil {
		t.Fatal("event B received another event's message")
	}
}
