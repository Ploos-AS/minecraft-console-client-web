package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestQualifyProtocolFixture(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		for i, want := range []struct {
			command   string
			requestID string
		}{
			{command: "Authenticate", requestID: "qual-auth"},
			{command: "GetItemTypeMappings", requestID: "qual-items"},
		} {
			var got command
			if err := conn.ReadJSON(&got); err != nil {
				t.Errorf("read command %d: %v", i, err)
				return
			}
			if got.Command != want.command || got.RequestID != want.requestID {
				t.Errorf("command %d = %#v, want command=%q requestId=%q", i, got, want.command, want.requestID)
				return
			}
			if i == 0 {
				if len(got.Parameters) != 1 || got.Parameters[0] != "secret" {
					t.Errorf("unexpected auth parameters: %#v", got.Parameters)
					return
				}
			}

			data, _ := json.Marshal(response{Success: true, RequestID: want.requestID})
			if err := conn.WriteJSON(envelope{Event: "OnWsCommandResponse", Data: string(data)}); err != nil {
				t.Errorf("write response %d: %v", i, err)
				return
			}
		}
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	if err := qualify(context.Background(), url, "secret", 2*time.Second); err != nil {
		t.Fatalf("qualify: %v", err)
	}
}

func TestQualifyRejectsFailedAuthentication(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var got command
		if err := conn.ReadJSON(&got); err != nil {
			return
		}
		data, _ := json.Marshal(response{Success: false, RequestID: got.RequestID, Message: "authentication failed"})
		_ = conn.WriteJSON(envelope{Event: "OnWsCommandResponse", Data: string(data)})
	}))
	defer server.Close()

	url := "ws" + strings.TrimPrefix(server.URL, "http")
	if err := qualify(context.Background(), url, "wrong", 2*time.Second); err == nil {
		t.Fatal("qualify succeeded with failed authentication")
	}
}
