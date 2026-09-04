package mcc

import (
	"encoding/json"
	"testing"
)

func TestParseBrowserCommand(t *testing.T) {
	command, err := parseBrowserCommand([]byte(`{"type":"command","id":"ui-7","command":"GetItemTypeMappings","parameters":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if command.ID != "ui-7" || command.Command != "GetItemTypeMappings" {
		t.Fatalf("unexpected command: %#v", command)
	}
}

func TestNormalizedEventDecodesNestedData(t *testing.T) {
	message, response, err := normalizedEvent([]byte(`{"event":"OnChatRaw","data":"{\"text\":\"hello\"}"}`))
	if err != nil {
		t.Fatal(err)
	}
	if response != nil {
		t.Fatal("ordinary event was treated as command response")
	}
	var got map[string]any
	if err := json.Unmarshal(message.payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["type"] != "event" || got["event"] != "OnChatRaw" {
		t.Fatalf("unexpected normalized event: %#v", got)
	}
	data, ok := got["data"].(map[string]any)
	if !ok || data["text"] != "hello" {
		t.Fatalf("unexpected normalized data: %#v", got["data"])
	}
}

func TestNormalizedCommandResponseIsSeparated(t *testing.T) {
	message, response, err := normalizedEvent([]byte(`{"event":"OnWsCommandResponse","data":"{\"success\":true,\"requestId\":\"mcc-web-4\"}"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(message.payload) != 0 {
		t.Fatalf("command response leaked as broadcast payload: %q", message.payload)
	}
	if response == nil || !response.Success || response.RequestID != "mcc-web-4" {
		t.Fatalf("unexpected response: %#v", response)
	}
}
