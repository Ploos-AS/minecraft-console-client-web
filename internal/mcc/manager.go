package mcc

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Manager owns one persistent authenticated MCC connection and fans upstream
// events out to all subscribed browser clients.
type Manager struct {
	URL      string
	Password string
	Log      *slog.Logger
	Dialer   *websocket.Dialer

	mu          sync.RWMutex
	status      Status
	subscribers map[chan browserMessage]struct{}
	commands    chan browserMessage
}

func NewManager(url, password string, log *slog.Logger) *Manager {
	if log == nil {
		log = slog.Default()
	}
	return &Manager{
		URL:         url,
		Password:    password,
		Log:         log,
		Dialer:      websocket.DefaultDialer,
		status:      Status{State: StateDisconnected},
		subscribers: make(map[chan browserMessage]struct{}),
		commands:    make(chan browserMessage, 64),
	}
}

func (m *Manager) Status() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *Manager) setStatus(state State, attempts int, err error) {
	m.mu.Lock()
	m.status.State = state
	m.status.Attempts = attempts
	if err != nil {
		m.status.LastError = err.Error()
	} else {
		m.status.LastError = ""
	}
	if state == StateConnected {
		m.status.ConnectedAt = time.Now().UTC()
	} else if state == StateDisconnected {
		m.status.ConnectedAt = time.Time{}
	}
	status := m.status
	m.mu.Unlock()
	m.broadcast(statusMessage(status))
}

func (m *Manager) Subscribe() (<-chan browserMessage, func()) {
	ch := make(chan browserMessage, 64)
	m.mu.Lock()
	m.subscribers[ch] = struct{}{}
	status := m.status
	m.mu.Unlock()
	ch <- statusMessage(status)
	return ch, func() {
		m.mu.Lock()
		delete(m.subscribers, ch)
		m.mu.Unlock()
	}
}

func (m *Manager) Send(ctx context.Context, messageType int, payload []byte) error {
	msg := browserMessage{messageType: messageType, payload: append([]byte(nil), payload...)}
	select {
	case m.commands <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) broadcast(msg browserMessage) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for ch := range m.subscribers {
		select {
		case ch <- msg:
		default:
			m.Log.Warn("dropping MCC event for slow browser subscriber")
		}
	}
}

func (m *Manager) Run(ctx context.Context) error {
	attempt := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt++
		m.setStatus(StateConnecting, attempt, nil)
		conn, _, err := m.Dialer.DialContext(ctx, m.URL, http.Header{})
		if err != nil {
			m.setStatus(StateDisconnected, attempt, err)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil {
				return err
			}
			continue
		}

		m.setStatus(StateAuthenticating, attempt, nil)
		s := Session{Password: m.Password}
		if err := s.authenticate(conn); err != nil {
			_ = conn.Close()
			m.setStatus(StateDisconnected, attempt, err)
			if err := waitReconnect(ctx, reconnectDelay(attempt)); err != nil {
				return err
			}
			continue
		}

		attempt = 0
		m.setStatus(StateConnected, 0, nil)
		err = m.runConnected(ctx, conn)
		_ = conn.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m.setStatus(StateDisconnected, 1, err)
	}
}

func (m *Manager) runConnected(ctx context.Context, conn *websocket.Conn) error {
	errCh := make(chan error, 2)
	go func() {
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			m.broadcast(browserMessage{messageType: messageType, payload: append([]byte(nil), payload...)})
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-m.commands:
				if err := conn.WriteMessage(msg.messageType, msg.payload); err != nil {
					errCh <- fmt.Errorf("write MCC command: %w", err)
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

func statusMessage(status Status) browserMessage {
	payload, _ := jsonMarshalStatus(status)
	return browserMessage{messageType: websocket.TextMessage, payload: payload}
}
