package mcc

import (
	"testing"
	"time"
)

func TestReconnectDelayBounds(t *testing.T) {
	tests := []struct {
		attempt int
		min     time.Duration
		max     time.Duration
	}{
		{attempt: 1, min: time.Second, max: 1500 * time.Millisecond},
		{attempt: 2, min: 2 * time.Second, max: 2500 * time.Millisecond},
		{attempt: 6, min: 32 * time.Second, max: 32500 * time.Millisecond},
		{attempt: 20, min: 32 * time.Second, max: 32500 * time.Millisecond},
	}
	for _, test := range tests {
		delay := reconnectDelay(test.attempt)
		if delay < test.min || delay >= test.max {
			t.Fatalf("reconnectDelay(%d) = %s, want [%s,%s)", test.attempt, delay, test.min, test.max)
		}
	}
}

func TestStatusTransitions(t *testing.T) {
	session := &Session{}
	session.setStatus(StateConnecting, 2, nil)
	status := session.Status()
	if status.State != StateConnecting || status.Attempts != 2 || status.LastError != "" {
		t.Fatalf("unexpected connecting status: %+v", status)
	}

	session.setStatus(StateConnected, 0, nil)
	status = session.Status()
	if status.State != StateConnected || status.ConnectedAt.IsZero() {
		t.Fatalf("unexpected connected status: %+v", status)
	}
}
