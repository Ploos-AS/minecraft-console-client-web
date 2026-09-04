package mcc

import "testing"

func TestParseEventAndDecodeCommandResponse(t *testing.T) {
	payload := []byte(`{"event":"OnWsCommandResponse","data":"{\"success\":true,\"requestId\":\"abc\",\"message\":\"ok\"}"}`)

	event, err := ParseEvent(payload)
	if err != nil {
		t.Fatalf("ParseEvent() error = %v", err)
	}
	response, err := DecodeCommandResponse(event)
	if err != nil {
		t.Fatalf("DecodeCommandResponse() error = %v", err)
	}
	if !response.Success || response.RequestID != "abc" || response.Message != "ok" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestParseEventRejectsMissingName(t *testing.T) {
	if _, err := ParseEvent([]byte(`{"data":"{}"}`)); err == nil {
		t.Fatal("ParseEvent() expected error")
	}
}

func TestDecodeCommandResponseRejectsWrongEvent(t *testing.T) {
	if _, err := DecodeCommandResponse(Event{Event: "OnGameJoined", Data: `"N/A"`}); err == nil {
		t.Fatal("DecodeCommandResponse() expected error")
	}
}
