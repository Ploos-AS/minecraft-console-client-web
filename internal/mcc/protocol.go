package mcc

import (
	"encoding/json"
	"fmt"
)

// Command is the JSON command envelope accepted by MCC's WebSocketBot.
type Command struct {
	Command    string `json:"command"`
	RequestID  string `json:"requestId"`
	Parameters []any  `json:"parameters"`
}

// Event is the outer event envelope emitted by MCC's WebSocketBot.
// Data is itself a JSON-encoded string and must be decoded separately.
type Event struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// CommandResponse is the payload of OnWsCommandResponse.
type CommandResponse struct {
	Success   bool   `json:"success"`
	RequestID string `json:"requestId"`
	Message   string `json:"message,omitempty"`
}

// ParseEvent validates and decodes an MCC event envelope.
func ParseEvent(payload []byte) (Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, fmt.Errorf("decode MCC event: %w", err)
	}
	if event.Event == "" {
		return Event{}, fmt.Errorf("decode MCC event: missing event name")
	}
	return event, nil
}

// DecodeData decodes the event's nested JSON data string into dst.
func (e Event) DecodeData(dst any) error {
	if err := json.Unmarshal([]byte(e.Data), dst); err != nil {
		return fmt.Errorf("decode %s data: %w", e.Event, err)
	}
	return nil
}

// DecodeCommandResponse decodes an OnWsCommandResponse event.
func DecodeCommandResponse(event Event) (CommandResponse, error) {
	if event.Event != "OnWsCommandResponse" {
		return CommandResponse{}, fmt.Errorf("expected OnWsCommandResponse, got %s", event.Event)
	}
	var response CommandResponse
	if err := event.DecodeData(&response); err != nil {
		return CommandResponse{}, err
	}
	if response.RequestID == "" {
		return CommandResponse{}, fmt.Errorf("decode OnWsCommandResponse: missing requestId")
	}
	return response, nil
}
