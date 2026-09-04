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
		if !authenticateFixture(conn) {
			return
		}
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

	waitForState(t, manager, StateConnected, 2*time.Second)
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

func TestManagerFansOutOneUpstreamEventToMultipleSubscribers(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	sendEvent := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		if !authenticateFixture(conn) {
			return
		}
		<-sendEvent
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"event":"OnChatRaw","data":"{}"}`))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager := NewManager("ws"+strings.TrimPrefix(server.URL, "http"), "secret", nil)
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	waitForState(t, manager, StateConnected, 2*time.Second)

	a, unsubA := manager.subscribe()
	defer unsubA()
	b, unsubB := manager.subscribe()
	defer unsubB()
	drainInitialStatus(t, a)
	drainInitialStatus(t, b)
	close(sendEvent)

	want := `{"event":"OnChatRaw","data":"{}"}`
	if got := readTextPayload(t, a); got != want {
		t.Fatalf("subscriber A payload = %q, want %q", got, want)
	}
	if got := readTextPayload(t, b); got != want {
		t.Fatalf("subscriber B payload = %q, want %q", got, want)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("manager did not stop after cancellation")
	}
}

func TestManagerReconnectsAfterUpstreamDisconnect(t *testing.T) {
	var connections atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		n := connections.Add(1)
		if !authenticateFixture(conn) {
			return
		}
		if n == 1 {
			return
		}
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

	deadline := time.Now().Add(4 * time.Second)
	for (connections.Load() < 2 || manager.Status().State != StateConnected) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := connections.Load(); got < 2 {
		t.Fatalf("upstream connections = %d, want at least 2", got)
	}
	if got := manager.Status().State; got != StateConnected {
		t.Fatalf("manager state = %q, want %q", got, StateConnected)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("manager did not stop after cancellation")
	}
}

func authenticateFixture(conn *websocket.Conn) bool {
	var cmd Command
	if err := conn.ReadJSON(&cmd); err != nil {
		return false
	}
	data, _ := json.Marshal(CommandResponse{Success: true, RequestID: cmd.RequestID})
	return conn.WriteJSON(Event{Event: "OnWsCommandResponse", Data: string(data)}) == nil
}

func waitForState(t *testing.T, manager *Manager, state State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for manager.Status().State != state && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := manager.Status().State; got != state {
		t.Fatalf("manager state = %q, want %q", got, state)
	}
}

func drainInitialStatus(t *testing.T, ch <-chan browserMessage) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial status")
	}
}

func readTextPayload(t *testing.T, ch <-chan browserMessage) string {
	t.Helper()
	select {
	case msg := <-ch:
		if msg.messageType != websocket.TextMessage {
			t.Fatalf("message type = %d, want text", msg.messageType)
		}
		return string(msg.payload)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for fan-out event")
		return ""
	}
}
