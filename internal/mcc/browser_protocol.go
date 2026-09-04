package mcc

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
)

// BrowserCommand is the stable command envelope accepted from the WebAdmin UI.
// ID belongs to the browser client and is preserved in the corresponding response.
type BrowserCommand struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	Command    string `json:"command"`
	Parameters []any  `json:"parameters,omitempty"`
}

type browserCommandResponse struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

type browserEvent struct {
	Type  string `json:"type"`
	Event string `json:"event"`
	Data  any    `json:"data"`
}

func parseBrowserCommand(payload []byte) (BrowserCommand, error) {
	var command BrowserCommand
	if err := json.Unmarshal(payload, &command); err != nil {
		return BrowserCommand{}, fmt.Errorf("decode browser command: %w", err)
	}
	if command.Type != "command" {
		return BrowserCommand{}, fmt.Errorf("unsupported browser message type %q", command.Type)
	}
	if command.ID == "" {
		return BrowserCommand{}, fmt.Errorf("browser command missing id")
	}
	if command.Command == "" {
		return BrowserCommand{}, fmt.Errorf("browser command missing command")
	}
	if command.Parameters == nil {
		command.Parameters = []any{}
	}
	return command, nil
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

func protocolErrorMessage(message string) browserMessage {
	return jsonBrowserMessage(map[string]any{"type": "error", "message": message})
}

func jsonBrowserMessage(value any) browserMessage {
	payload, _ := json.Marshal(value)
	return browserMessage{messageType: websocket.TextMessage, payload: payload}
}
