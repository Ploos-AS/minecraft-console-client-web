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

type config struct { ListenAddr, MCCURL, MCCPassword string }
type app struct { cfg config; log *slog.Logger; manager *mcc.Manager; upgrader websocket.Upgrader }

func main() {
	cfg:=config{ListenAddr:env("MCC_WEB_LISTEN",":8080"), MCCURL:env("MCC_WS_URL","ws://mcc:8043/"), MCCPassword:os.Getenv("MCC_WS_PASSWORD")}
	if cfg.MCCPassword=="" { fmt.Fprintln(os.Stderr,"MCC_WS_PASSWORD is required"); os.Exit(2) }
	logger:=slog.New(slog.NewJSONHandler(os.Stdout,nil)); manager:=mcc.NewManager(cfg.MCCURL,cfg.MCCPassword,logger)
	a:=&app{cfg:cfg,log:logger,manager:manager,upgrader:websocket.Upgrader{ReadBufferSize:4096,WriteBufferSize:4096,CheckOrigin:sameOrigin}}
	mux:=http.NewServeMux(); mux.HandleFunc("GET /api/healthz",a.healthz); mux.HandleFunc("GET /api/status",a.status); mux.HandleFunc("GET /ws",a.bridge)
	static,err:=fs.Sub(webFS,"web"); if err!=nil{panic(err)}; mux.Handle("/",http.FileServer(http.FS(static)))
	srv:=&http.Server{Addr:cfg.ListenAddr,Handler:securityHeaders(mux),ReadHeaderTimeout:5*time.Second,IdleTimeout:60*time.Second}
	ctx,stop:=signal.NotifyContext(context.Background(),syscall.SIGINT,syscall.SIGTERM); defer stop()
	go func(){ if err:=manager.Run(ctx); err!=nil && !errors.Is(err,context.Canceled){logger.Error("MCC manager stopped","error",err)} }()
	go func(){ logger.Info("minecraft-console-client-web starting","listen",cfg.ListenAddr); if err:=srv.ListenAndServe();err!=nil&&!errors.Is(err,http.ErrServerClosed){logger.Error("http server failed","error",err);os.Exit(1)} }()
	<-ctx.Done(); shutdownCtx,cancel:=context.WithTimeout(context.Background(),10*time.Second); defer cancel(); _=srv.Shutdown(shutdownCtx)
}
func (a *app) healthz(w http.ResponseWriter,_ *http.Request){w.Header().Set("Content-Type","application/json");_,_=w.Write([]byte(`{"status":"ok"}`))}
func (a *app) status(w http.ResponseWriter,_ *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]any{"service":"minecraft-console-client-web","mode":"shared-session-manager","mcc":a.manager.Status()})}
func (a *app) bridge(w http.ResponseWriter,r *http.Request){browser,err:=a.upgrader.Upgrade(w,r,nil);if err!=nil{a.log.Warn("browser websocket upgrade failed","error",err);return};defer browser.Close();if err:=a.manager.ServeBrowser(r.Context(),browser);err!=nil&&!errors.Is(err,context.Canceled)&&!websocket.IsCloseError(err,websocket.CloseNormalClosure,websocket.CloseGoingAway){a.log.Debug("browser websocket closed","error",err)}}
func sameOrigin(r *http.Request) bool { origin:=r.Header.Get("Origin"); if origin==""{return true}; u,err:=url.Parse(origin); return err==nil&&strings.EqualFold(u.Host,r.Host) }
func securityHeaders(next http.Handler) http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("X-Content-Type-Options","nosniff");w.Header().Set("X-Frame-Options","DENY");w.Header().Set("Referrer-Policy","no-referrer");w.Header().Set("Content-Security-Policy","default-src 'self'; connect-src 'self' ws: wss:; style-src 'self'; script-src 'self'");next.ServeHTTP(w,r)})}
func env(key,fallback string)string{if value:=strings.TrimSpace(os.Getenv(key));value!=""{return value};return fallback}
