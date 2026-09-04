package mcc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const authRequestID = "mcc-web-auth"

// State describes the upstream MCC WebSocket connection state.
type State string

const (
	StateDisconnected   State = "disconnected"
	StateConnecting     State = "connecting"
	StateAuthenticating State = "authenticating"
	StateConnected      State = "connected"
)

// Status is the externally visible state of one browser-to-MCC session.
type Status struct {
	State       State     `json:"state"`
	LastError   string    `json:"lastError,omitempty"`
	ConnectedAt time.Time `json:"connectedAt,omitempty"`
	Attempts    int       `json:"attempts"`
}

// Session bridges one browser WebSocket to MCC while keeping the browser
// connection alive across transient MCC disconnects.
type Session struct {
	URL      string
	Password string
	Log      *slog.Logger
	Dialer   *websocket.Dialer

	mu     sync.RWMutex
	status Status
}

func (s *Session) Status() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Session) setStatus(state State, attempts int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = state
	s.status.Attempts = attempts
	if err != nil {
		s.status.LastError = err.Error()
	} else {
		s.status.LastError = ""
	}
	if state == StateConnected {
		s.status.ConnectedAt = time.Now().UTC()
	} else if state == StateDisconnected {
		s.status.ConnectedAt = time.Time{}
	}
}

// Run starts the bidirectional bridge. Browser messages are buffered while MCC
// reconnects and are delivered after the next successful authentication.
func (s *Session) Run(ctx context.Context, browser *websocket.Conn) error {
	if s.Dialer == nil {
		s.Dialer = websocket.DefaultDialer
	}
	if s.Log == nil {
		s.Log = slog.Default()
	}

	toMCC := make(chan browserMessage, 64)
	toBrowser := make(chan browserMessage, 64)
	errCh := make(chan error, 2)

	go readBrowser(ctx, browser, toMCC, errCh)
	go writeBrowser(ctx, browser, toBrowser, errCh)

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		default:
		}

		attempt++
		s.setStatus(StateConnecting, attempt, nil)
		s.emitStatus(toBrowser)

		upstream, _, err := s.Dialer.DialContext(ctx, s.URL, http.Header{})
		if err != nil {
			s.setStatus(StateDisconnected, attempt, err)
			s.emitStatus(toBrowser)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil {
				return err
			}
			continue
		}

		s.setStatus(StateAuthenticating, attempt, nil)
		s.emitStatus(toBrowser)
		if err := s.authenticate(upstream); err != nil {
			_ = upstream.Close()
			s.setStatus(StateDisconnected, attempt, err)
			s.emitStatus(toBrowser)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil {
				return err
			}
			continue
		}

		attempt = 0
		s.setStatus(StateConnected, attempt, nil)
		s.emitStatus(toBrowser)

		err = bridgeUpstream(ctx, upstream, toMCC, toBrowser)
		_ = upstream.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		s.setStatus(StateDisconnected, 1, err)
		s.emitStatus(toBrowser)
	}
}

type browserMessage struct {
	messageType int
	payload     []byte
}

func readBrowser(ctx context.Context, conn *websocket.Conn, out chan<- browserMessage, errCh chan<- error) {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
			return
		}
		select {
		case out <- browserMessage{messageType: messageType, payload: payload}:
		case <-ctx.Done():
			return
		}
	}
}

func writeBrowser(ctx context.Context, conn *websocket.Conn, in <-chan browserMessage, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case message := <-in:
			if err := conn.WriteMessage(message.messageType, message.payload); err != nil {
				select {
				case errCh <- err:
				case <-ctx.Done():
				}
				return
			}
		}
	}
}

func (s *Session) authenticate(conn *websocket.Conn) error {
	command := Command{Command: "Authenticate", RequestID: authRequestID, Parameters: []any{s.Password}}
	if err := conn.WriteJSON(command); err != nil {
		return fmt.Errorf("send MCC authentication: %w", err)
	}
	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("set MCC authentication deadline: %w", err)
	}
	defer conn.SetReadDeadline(time.Time{})

	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read MCC authentication response: %w", err)
		}
		if messageType != websocket.TextMessage {
			continue
		}
		event, err := ParseEvent(payload)
		if err != nil || event.Event != "OnWsCommandResponse" {
			continue
		}
		response, err := DecodeCommandResponse(event)
		if err != nil || response.RequestID != authRequestID {
			continue
		}
		if !response.Success {
			if response.Message == "" {
				response.Message = "authentication rejected"
			}
			return errors.New(response.Message)
		}
		return nil
	}
}

func bridgeUpstream(ctx context.Context, upstream *websocket.Conn, toMCC <-chan browserMessage, toBrowser chan<- browserMessage) error {
	errCh := make(chan error, 2)
	go func() {
		for {
			messageType, payload, err := upstream.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			select {
			case toBrowser <- browserMessage{messageType: messageType, payload: payload}:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case message := <-toMCC:
				if err := upstream.WriteMessage(message.messageType, message.payload); err != nil {
					errCh <- err
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *Session) emitStatus(out chan<- browserMessage) {
	payload, err := json.Marshal(map[string]any{
		"type":   "mcc-web-status",
		"status": s.Status(),
	})
	if err != nil {
		return
	}
	select {
	case out <- browserMessage{messageType: websocket.TextMessage, payload: payload}:
	default:
	}
}

func reconnectDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	exponent := math.Min(float64(attempt-1), 5)
	base := time.Second * time.Duration(1<<int(exponent))
	jitter := time.Duration(rand.IntN(500)) * time.Millisecond
	return base + jitter
}

func waitReconnect(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
