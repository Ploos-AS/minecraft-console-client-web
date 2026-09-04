package mcc

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

// BrowserRequest is the stable browser-to-WebAdmin envelope. Command requests
// map to WebSocketBot JSON procedures. Text requests map to WebSocketBot's
// plain-text path, which sends chat or an MCC internal command when prefixed
// with '/'. Session actions are handled locally by WebAdmin.
type BrowserRequest struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Command    string `json:"command,omitempty"`
	Parameters []any  `json:"parameters,omitempty"`
	Text       string `json:"text,omitempty"`
	Action     string `json:"action,omitempty"`
}

// BrowserCommand remains as a compatibility alias for code that builds typed
// WebSocketBot procedure requests.
type BrowserCommand = BrowserRequest

type browserCommandResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type browserTextResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type browserSessionActionResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Action  string `json:"action"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type browserEvent struct {
	Type  string `json:"type"`
	Event string `json:"event"`
	Data  any    `json:"data"`
}

func parseBrowserRequest(payload []byte) (BrowserRequest, error) {
	var request BrowserRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return BrowserRequest{}, fmt.Errorf("decode browser request: %w", err)
	}
	if request.ID == "" {
		return BrowserRequest{}, fmt.Errorf("browser request missing id")
	}
	switch request.Type {
	case "command":
		if request.Command == "" {
			return BrowserRequest{}, fmt.Errorf("browser command missing command")
		}
		if request.Parameters == nil {
			request.Parameters = []any{}
		}
	case "text":
		if request.Text == "" {
			return BrowserRequest{}, fmt.Errorf("browser text request missing text")
		}
	case "session-action":
		if request.Action == "" {
			return BrowserRequest{}, fmt.Errorf("browser session action missing action")
		}
		if request.Action != "reconnect" {
			return BrowserRequest{}, fmt.Errorf("unsupported browser session action %q", request.Action)
		}
	default:
		return BrowserRequest{}, fmt.Errorf("unsupported browser message type %q", request.Type)
	}
	return request, nil
}

func parseBrowserCommand(payload []byte) (BrowserCommand, error) {
	request, err := parseBrowserRequest(payload)
	if err != nil {
		return BrowserCommand{}, err
	}
	if request.Type != "command" {
		return BrowserCommand{}, fmt.Errorf("unsupported browser message type %q", request.Type)
	}
	return request, nil
}

func normalizedEvent(payload []byte) (browserMessage, *CommandResponse, error) {
	event, err := ParseEvent(payload)
	if err != nil {
		return browserMessage{}, nil, err
	}
	if event.Event == "OnWsCommandResponse" {
		response, err := DecodeCommandResponse(event)
		if err != nil {
			return browserMessage{}, nil, err
		}
		return browserMessage{}, &response, nil
	}
	var data any
	if event.Data != "" {
		if err := json.Unmarshal([]byte(event.Data), &data); err != nil {
			data = event.Data
		}
	}
	return jsonBrowserMessage(browserEvent{Type: "event", Event: event.Event, Data: data}), nil, nil
}

func commandResponseMessage(id string, response CommandResponse) browserMessage {
	return jsonBrowserMessage(browserCommandResponse{Type: "command-response", ID: id, Success: response.Success, Message: response.Message})
}

func textResponseMessage(id string, success bool, message string) browserMessage {
	return jsonBrowserMessage(browserTextResponse{Type: "text-response", ID: id, Success: success, Message: message})
}

func sessionActionResponseMessage(id, action string, success bool, message string) browserMessage {
	return jsonBrowserMessage(browserSessionActionResponse{Type: "session-action-response", ID: id, Action: action, Success: success, Message: message})
}

func protocolErrorMessage(message string) browserMessage {
	return jsonBrowserMessage(map[string]any{"type": "error", "message": message})
}

func jsonBrowserMessage(value any) browserMessage {
	payload, _ := json.Marshal(value)
	return browserMessage{messageType: websocket.TextMessage, payload: payload}
}
