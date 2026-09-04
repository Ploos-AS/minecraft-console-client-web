package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testAuthApp() *app {
	return &app{
		auth:      newAuthStore("admin", "secret"),
		log:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		loginPage: []byte("login"),
	}
}

func TestRequireAuthRejectsAPIWithoutSession(t *testing.T) {
	a := testAuthApp()
	h := a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/api/status", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestRequireAuthRedirectsUIWithoutSession(t *testing.T) {
	a := testAuthApp()
	h := a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}
	if got := rr.Header().Get("Location"); got != "/login" {
		t.Fatalf("location = %q, want /login", got)
	}
}

func TestLoginCreatesAuthenticatedSession(t *testing.T) {
	a := testAuthApp()
	form := url.Values{"username": {"admin"}, "password": {"secret"}}
	r := httptest.NewRequest("POST", "/api/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	a.login(rr, r)
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusSeeOther)
	}

	response := rr.Result()
	defer response.Body.Close()
	cookies := response.Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookieName {
		t.Fatalf("expected one %s cookie, got %#v", sessionCookieName, cookies)
	}

	protected := a.requireAuth(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	authenticated := httptest.NewRequest("GET", "/api/status", nil)
	authenticated.AddCookie(cookies[0])
	authenticatedRR := httptest.NewRecorder()
	protected.ServeHTTP(authenticatedRR, authenticated)
	if authenticatedRR.Code != http.StatusNoContent {
		t.Fatalf("authenticated status = %d, want %d", authenticatedRR.Code, http.StatusNoContent)
	}
}
