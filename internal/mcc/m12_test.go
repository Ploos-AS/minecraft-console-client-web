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

func TestParseBrowserSessionReconnect(t *testing.T) {
	request, err := parseBrowserRequest([]byte(`{"type":"session-action","id":"ui-session-1","action":"reconnect"}`))
	if err != nil {
		t.Fatal(err)
	}
	if request.Type != "session-action" || request.ID != "ui-session-1" || request.Action != "reconnect" {
		t.Fatalf("unexpected session action: %#v", request)
	}
}

func TestParseBrowserSessionReconnectRejectsUnsupportedAction(t *testing.T) {
	if _, err := parseBrowserRequest([]byte(`{"type":"session-action","id":"ui-session-2","action":"restart"}`)); err == nil {
		t.Fatal("unsupported session action was accepted")
	}
}

func TestManagerSessionReconnectFailsPendingAndReconnects(t *testing.T) {
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
	defer cancel()
	manager := NewManager("ws"+strings.TrimPrefix(server.URL, "http"), "secret", nil)
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	waitForState(t, manager, StateConnected, 2*time.Second)

	commandReplies := make(chan browserMessage, 1)
	if err := manager.send(ctx, browserRequest{
		request: BrowserRequest{Type: "command", ID: "ui-command-1", Command: "GetUsername", Parameters: []any{}},
		reply:   commandReplies,
	}); err != nil {
		t.Fatal(err)
	}

	actionReplies := make(chan browserMessage, 1)
	if err := manager.send(ctx, browserRequest{
		request: BrowserRequest{Type: "session-action", ID: "ui-session-3", Action: "reconnect"},
		reply:   actionReplies,
	}); err != nil {
		t.Fatal(err)
	}

	select {
	case reply := <-actionReplies:
		var got map[string]any
		if err := json.Unmarshal(reply.payload, &got); err != nil {
			t.Fatal(err)
		}
		if got["type"] != "session-action-response" || got["id"] != "ui-session-3" || got["action"] != "reconnect" || got["success"] != true {
			t.Fatalf("unexpected session action response: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for session action response")
	}

	select {
	case reply := <-commandReplies:
		var got map[string]any
		if err := json.Unmarshal(reply.payload, &got); err != nil {
			t.Fatal(err)
		}
		if got["type"] != "command-response" || got["id"] != "ui-command-1" || got["success"] != false || got["message"] != "MCC reconnect requested" {
			t.Fatalf("unexpected pending-command failure: %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending command failure")
	}

	deadline := time.Now().Add(2 * time.Second)
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
