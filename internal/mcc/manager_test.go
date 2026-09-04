package mcc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestManagerConnectsWithoutBrowserSubscribers(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		connections.Add(1)
		var cmd Command
		if err := conn.ReadJSON(&cmd); err != nil {
			return
		}
		data, _ := json.Marshal(CommandResponse{Success: true, RequestID: cmd.RequestID})
		_ = conn.WriteJSON(Event{Event: "OnWsCommandResponse", Data: string(data)})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	manager := NewManager("ws"+strings.TrimPrefix(server.URL, "http"), "secret", nil)
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for manager.Status().State != StateConnected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := manager.Status().State; got != StateConnected {
		t.Fatalf("manager state = %q, want %q", got, StateConnected)
	}
	if got := connections.Load(); got != 1 {
		t.Fatalf("upstream connections = %d, want 1", got)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("manager did not stop after cancellation")
	}
}
