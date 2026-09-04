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
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Ploos-AS/minecraft-console-client-web/internal/mcc"
	"github.com/gorilla/websocket"
)

//go:embed web/*
var webFS embed.FS

type config struct {
	ListenAddr     string
	MCCURL         string
	MCCPassword    string
	WebUsername    string
	WebPassword    string
}

type app struct {
	cfg       config
	log       *slog.Logger
	manager   *mcc.Manager
	auth      *authStore
	loginPage []byte
	upgrader  websocket.Upgrader
}

func main() {
	cfg := config{
		ListenAddr:  env("MCC_WEB_LISTEN", ":8080"),
		MCCURL:      env("MCC_WS_URL", "ws://mcc:8043/"),
		MCCPassword: os.Getenv("MCC_WS_PASSWORD"),
		WebUsername: env("MCC_WEB_USERNAME", "admin"),
		WebPassword: os.Getenv("MCC_WEB_PASSWORD"),
	}
	if cfg.MCCPassword == "" {
		fmt.Fprintln(os.Stderr, "MCC_WS_PASSWORD is required")
		os.Exit(2)
	}
	if cfg.WebPassword == "" {
		fmt.Fprintln(os.Stderr, "MCC_WEB_PASSWORD is required")
		os.Exit(2)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	manager := mcc.NewManager(cfg.MCCURL, cfg.MCCPassword, logger)
	static, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	loginPage, err := fs.ReadFile(static, "login.html")
	if err != nil {
		panic(err)
	}
	a := &app{
		cfg:       cfg,
		log:       logger,
		manager:   manager,
		auth:      newAuthStore(cfg.WebUsername, cfg.WebPassword),
		loginPage: loginPage,
		upgrader:  websocket.Upgrader{ReadBufferSize: 4096, WriteBufferSize: 4096, CheckOrigin: sameOrigin},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/healthz", a.healthz)
	mux.HandleFunc("GET /login", a.loginPageHandler)
	mux.HandleFunc("POST /api/login", a.login)
	mux.Handle("GET /login.css", http.FileServer(http.FS(static)))
	mux.Handle("GET /api/status", a.requireAuth(http.HandlerFunc(a.status)))
	mux.Handle("POST /api/logout", a.requireAuth(http.HandlerFunc(a.logout)))
	mux.Handle("GET /ws", a.requireAuth(http.HandlerFunc(a.bridge)))
	mux.Handle("/", a.requireAuth(http.FileServer(http.FS(static))))

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           securityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		if err := manager.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("MCC manager stopped", "error", err)
		}
	}()
	go func() {
		logger.Info("minecraft-console-client-web starting", "listen", cfg.ListenAddr)
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

func (a *app) loginPageHandler(w http.ResponseWriter, r *http.Request) {
	if a.auth.authenticated(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(a.loginPage)
}

func (a *app) login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid login request", http.StatusBadRequest)
		return
	}
	if !a.auth.validCredentials(r.FormValue("username"), r.FormValue("password")) {
		a.log.Warn("WebAdmin login rejected", "remote", r.RemoteAddr)
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	token, err := a.auth.createSession()
	if err != nil {
		a.log.Error("failed to create WebAdmin session", "error", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	setSessionCookie(w, r, token, a.auth.ttl)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (a *app) logout(w http.ResponseWriter, r *http.Request) {
	a.auth.revoke(r)
	clearSessionCookie(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (a *app) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.auth.authenticated(r) {
			next.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/ws" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
}

func (a *app) status(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"service": "minecraft-console-client-web", "mode": "shared-session-manager", "mcc": a.manager.Status()})
}

func (a *app) bridge(w http.ResponseWriter, r *http.Request) {
	browser, err := a.upgrader.Upgrade(w, r, nil)
	if err != nil {
		a.log.Warn("browser websocket upgrade failed", "error", err)
		return
	}
	defer browser.Close()
	if err := a.manager.ServeBrowser(r.Context(), browser); err != nil && !errors.Is(err, context.Canceled) && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		a.log.Debug("browser websocket closed", "error", err)
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
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
