package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
)

type command struct {
	Command    string `json:"command"`
	RequestID  string `json:"requestId"`
	Parameters []any  `json:"parameters"`
}

type envelope struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type response struct {
	Success   bool   `json:"success"`
	RequestID string `json:"requestId"`
	Message   string `json:"message"`
}

func main() {
	url := flag.String("url", env("MCC_WS_URL", "ws://127.0.0.1:8043/"), "MCC WebSocketBot URL")
	password := flag.String("password", os.Getenv("MCC_WS_PASSWORD"), "MCC WebSocketBot password (prefer MCC_WS_PASSWORD)")
	timeout := flag.Duration("timeout", 10*time.Second, "qualification timeout")
	flag.Parse()

	if *password == "" {
		fmt.Fprintln(os.Stderr, "MCC_WS_PASSWORD (or -password) is required")
		os.Exit(2)
	}

	if err := qualify(context.Background(), *url, *password, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL runtime-qualification: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("PASS auth")
	fmt.Println("PASS command-response")
	fmt.Println("PASS runtime-qualification")
}

func qualify(parent context.Context, url, password string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	if err := conn.WriteJSON(command{Command: "Authenticate", RequestID: "qual-auth", Parameters: []any{password}}); err != nil {
		return fmt.Errorf("send authentication: %w", err)
	}
	if err := awaitResponse(conn, "qual-auth"); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}

	if err := conn.WriteJSON(command{Command: "GetItemTypeMappings", RequestID: "qual-items", Parameters: []any{}}); err != nil {
		return fmt.Errorf("send protocol probe: %w", err)
	}
	if err := awaitResponse(conn, "qual-items"); err != nil {
		return fmt.Errorf("protocol probe: %w", err)
	}
	return nil
}

func awaitResponse(conn *websocket.Conn, requestID string) error {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		var event envelope
		if err := json.Unmarshal(payload, &event); err != nil || event.Event != "OnWsCommandResponse" {
			continue
		}
		var result response
		if err := json.Unmarshal([]byte(event.Data), &result); err != nil {
			return fmt.Errorf("decode OnWsCommandResponse: %w", err)
		}
		if result.RequestID != requestID {
			continue
		}
		if !result.Success {
			return fmt.Errorf("request %s failed: %s", requestID, result.Message)
		}
		return nil
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
