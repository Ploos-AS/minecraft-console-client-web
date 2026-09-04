package mcc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type commandRequest struct {
	command BrowserCommand
	reply   chan browserMessage
}

type pendingCommand struct {
	browserID string
	reply     chan browserMessage
}

type Manager struct {
	URL         string
	Password    string
	Log         *slog.Logger
	Dialer      *websocket.Dialer
	mu          sync.RWMutex
	status      Status
	subscribers map[chan browserMessage]struct{}
	commands    chan commandRequest
	sequence    atomic.Uint64
}

func NewManager(url, password string, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{URL: url, Password: password, Log: log, Dialer: websocket.DefaultDialer, status: Status{State: StateDisconnected}, subscribers: make(map[chan browserMessage]struct{}), commands: make(chan commandRequest, 64)}
}

func (m *Manager) Status() Status { m.mu.RLock(); defer m.mu.RUnlock(); return m.status }
func (m *Manager) setStatus(state State, attempts int, err error) {
	m.mu.Lock()
	m.status.State = state
	m.status.Attempts = attempts
	if err != nil { m.status.LastError = err.Error() } else { m.status.LastError = "" }
	if state == StateConnected { m.status.ConnectedAt = time.Now().UTC() } else if state == StateDisconnected { m.status.ConnectedAt = time.Time{} }
	status := m.status
	m.mu.Unlock()
	m.broadcast(statusMessage(status))
}
func (m *Manager) subscribe() (chan browserMessage, func()) {
	ch := make(chan browserMessage, 64)
	m.mu.Lock(); m.subscribers[ch] = struct{}{}; status := m.status; m.mu.Unlock()
	ch <- statusMessage(status)
	return ch, func() { m.mu.Lock(); delete(m.subscribers, ch); m.mu.Unlock() }
}
func (m *Manager) send(ctx context.Context, request commandRequest) error {
	select { case m.commands <- request: return nil; case <-ctx.Done(): return ctx.Err() }
}
func (m *Manager) broadcast(msg browserMessage) {
	m.mu.RLock(); defer m.mu.RUnlock()
	for ch := range m.subscribers {
		select { case ch <- msg: default: m.Log.Warn("dropping MCC event for slow browser subscriber") }
	}
}

// ServeBrowser attaches one browser to the shared MCC session. Browser clients
// speak the normalized WebAdmin protocol rather than raw MCC WebSocket frames.
func (m *Manager) ServeBrowser(ctx context.Context, browser *websocket.Conn) error {
	events, unsubscribe := m.subscribe()
	defer unsubscribe()
	errCh := make(chan error, 2)
	go func() {
		for {
			mt, payload, err := browser.ReadMessage()
			if err != nil { errCh <- err; return }
			if mt != websocket.TextMessage { nonblockingReply(events, protocolErrorMessage("only text messages are supported")); continue }
			command, err := parseBrowserCommand(payload)
			if err != nil { nonblockingReply(events, protocolErrorMessage(err.Error())); continue }
			if m.Status().State != StateConnected {
				nonblockingReply(events, commandResponseMessage(command.ID, CommandResponse{Success: false, Message: "MCC is not connected"}))
				continue
			}
			if err := m.send(ctx, commandRequest{command: command, reply: events}); err != nil { return }
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done(): return
			case msg := <-events:
				if err := browser.WriteMessage(msg.messageType, msg.payload); err != nil { errCh <- err; return }
			}
		}
	}()
	select { case <-ctx.Done(): return ctx.Err(); case err := <-errCh: return err }
}

func (m *Manager) Run(ctx context.Context) error {
	attempt := 0
	for {
		if ctx.Err() != nil { return ctx.Err() }
		attempt++; m.setStatus(StateConnecting, attempt, nil)
		conn, _, err := m.Dialer.DialContext(ctx, m.URL, http.Header{})
		if err != nil {
			m.setStatus(StateDisconnected, attempt, err)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil { return err }
			continue
		}
		m.setStatus(StateAuthenticating, attempt, nil)
		s := Session{Password: m.Password}
		if err := s.authenticate(conn); err != nil {
			_ = conn.Close(); m.setStatus(StateDisconnected, attempt, err)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil { return err }
			continue
		}
		attempt = 0; m.setStatus(StateConnected, 0, nil)
		err = m.runConnected(ctx, conn); _ = conn.Close()
		if ctx.Err() != nil { return ctx.Err() }
		m.setStatus(StateDisconnected, 1, err)
	}
}
func (m *Manager) runConnected(ctx context.Context, conn *websocket.Conn) error {
	errCh := make(chan error, 2); incoming := make(chan []byte, 64); pending := make(map[string]pendingCommand)
	go func() {
		for {
			mt, payload, err := conn.ReadMessage()
			if err != nil { errCh <- err; return }
			if mt != websocket.TextMessage { continue }
			select { case incoming <- append([]byte(nil), payload...): case <-ctx.Done(): return }
		}
	}()
	failPending := func(message string) {
		for id, request := range pending { nonblockingReply(request.reply, commandResponseMessage(request.browserID, CommandResponse{Success: false, Message: message})); delete(pending, id) }
	}
	for {
		select {
		case <-ctx.Done(): failPending("MCC connection closed"); return ctx.Err()
		case err := <-errCh: failPending("MCC connection lost"); return err
		case request := <-m.commands:
			upstreamID := fmt.Sprintf("mcc-web-%d", m.sequence.Add(1))
			command := Command{Command: request.command.Command, RequestID: upstreamID, Parameters: request.command.Parameters}
			if err := conn.WriteJSON(command); err != nil {
				nonblockingReply(request.reply, commandResponseMessage(request.command.ID, CommandResponse{Success: false, Message: "failed to send command to MCC"})); failPending("MCC connection lost")
				return fmt.Errorf("write MCC command: %w", err)
			}
			pending[upstreamID] = pendingCommand{browserID: request.command.ID, reply: request.reply}
		case payload := <-incoming:
			message, response, err := normalizedEvent(payload)
			if err != nil { m.Log.Warn("ignoring malformed MCC event", "error", err); continue }
			if response == nil { m.broadcast(message); continue }
			request, ok := pending[response.RequestID]
			if !ok { m.Log.Debug("ignoring unmatched MCC command response", "request_id", response.RequestID); continue }
			delete(pending, response.RequestID); nonblockingReply(request.reply, commandResponseMessage(request.browserID, *response))
		}
	}
}
func statusMessage(status Status) browserMessage { payload, _ := json.Marshal(map[string]any{"type": "status", "status": status}); return browserMessage{messageType: websocket.TextMessage, payload: payload} }
func nonblockingReply(ch chan browserMessage, msg browserMessage) { select { case ch <- msg: default: } }
