package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed web/*
var webFS embed.FS

type config struct {
	ListenAddr  string
	MCCURL      string
	MCCPassword string
}

type app struct {
	cfg      config
	log      *slog.Logger
	upgrader websocket.Upgrader
}

type wsCommand struct {
	Command    string `json:"command"`
	RequestID  string `json:"requestId"`
	Parameters []any  `json:"parameters"`
}

func main() {
	cfg := config{
		ListenAddr:  env("MCC_WEB_LISTEN", ":8080"),
		MCCURL:      env("MCC_WS_URL", "ws://mcc:8043/"),
		MCCPassword: os.Getenv("MCC_WS_PASSWORD"),
	}
	if cfg.MCCPassword == "" {
		fmt.Fprintln(os.Stderr, "MCC_WS_PASSWORD is required")
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	a := &app{
		cfg: cfg,
		log: logger,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				return origin == "" || strings.Contains(origin, r.Host)
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", a.healthz)
	mux.HandleFunc("GET /api/status", a.status)
	mux.HandleFunc("GET /ws", a.bridge)

	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("minecraft-console-client-web starting", "listen", cfg.ListenAddr, "mcc_ws", cfg.MCCURL)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

func (a *app) healthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (a *app) status(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"service":      "minecraft-console-client-web",
		"mccWebSocket": a.cfg.MCCURL,
	})
}

func (a *app) bridge(w http.ResponseWriter, r *http.Request) {
	browser, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.Warn("browser websocket upgrade failed", "error", err)
		return
	}
	defer browser.Close()

	upstream, _, err := websocket.DefaultDialer.DialContext(r.Context(), a.cfg.MCCURL, nil)
	if err != nil {
		_ = browser.WriteJSON(map[string]any{"type": "error", "message": "unable to connect to MCC WebSocket"})
		a.log.Warn("MCC websocket connection failed", "error", err)
		return
	}
	defer upstream.Close()

	auth := wsCommand{Command: "Authenticate", RequestID: "mcc-web-auth", Parameters: []any{a.cfg.MCCPassword}}
	if err := upstream.WriteJSON(auth); err != nil {
		_ = browser.WriteJSON(map[string]any{"type": "error", "message": "unable to authenticate to MCC"})
		return
	}

	errCh := make(chan error, 2)
	go copyWebSocket(upstream, browser, errCh)
	go copyWebSocket(browser, upstream, errCh)
	if err := <-errCh; err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		a.log.Debug("websocket bridge closed", "error", err)
	}
}

func copyWebSocket(src, dst *websocket.Conn, errCh chan<- error) {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			errCh <- err
			return
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			errCh <- err
			return
		}
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; style-src 'self'; script-src 'self'")
		next.ServeHTTP(w, r)
	})
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
