package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthStoreCredentials(t *testing.T) {
	a := newAuthStore("admin", "secret")
	if !a.validCredentials("admin", "secret") {
		t.Fatal("expected valid credentials")
	}
	if a.validCredentials("admin", "wrong") {
		t.Fatal("accepted wrong password")
	}
	if a.validCredentials("wrong", "secret") {
		t.Fatal("accepted wrong username")
	}
}

func TestAuthStoreSessionLifecycle(t *testing.T) {
	a := newAuthStore("admin", "secret")
	token, err := a.createSession()
	if err != nil {
		t.Fatal(err)
	}

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(sessionCookie(token))
	if !a.authenticated(r) {
		t.Fatal("expected session to authenticate")
	}

	a.revoke(r)
	if a.authenticated(r) {
		t.Fatal("revoked session still authenticated")
	}
}

func TestAuthStoreRejectsExpiredSession(t *testing.T) {
	a := newAuthStore("admin", "secret")
	token, err := a.createSession()
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.sessions[token] = session{expires: time.Now().Add(-time.Minute)}
	a.mu.Unlock()

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(sessionCookie(token))
	if a.authenticated(r) {
		t.Fatal("expired session authenticated")
	}
}

func TestSecureRequestHonorsTLSAndForwardedProto(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	if secureRequest(r) {
		t.Fatal("plain request reported secure")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !secureRequest(r) {
		t.Fatal("forwarded HTTPS request not reported secure")
	}
}

func sessionCookie(token string) *http.Cookie {
	return &http.Cookie{Name: sessionCookieName, Value: token}
}
