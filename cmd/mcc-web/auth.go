package main

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "mcc_web_session"

type session struct {
	expires time.Time
}

type authStore struct {
	username string
	password string
	ttl      time.Duration
	mu       sync.Mutex
	sessions map[string]session
}

func newAuthStore(username, password string) *authStore {
	return &authStore{
		username: username,
		password: password,
		ttl:      12 * time.Hour,
		sessions: make(map[string]session),
	}
}

func (a *authStore) validCredentials(username, password string) bool {
	return subtle.ConstantTimeCompare([]byte(username), []byte(a.username)) == 1 &&
		subtle.ConstantTimeCompare([]byte(password), []byte(a.password)) == 1
}

func (a *authStore) createSession() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	a.mu.Lock()
	a.sessions[token] = session{expires: time.Now().Add(a.ttl)}
	a.pruneLocked(time.Now())
	a.mu.Unlock()
	return token, nil
}

func (a *authStore) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[cookie.Value]
	if !ok || now.After(s.expires) {
		delete(a.sessions, cookie.Value)
		return false
	}
	return true
}

func (a *authStore) revoke(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	a.mu.Lock()
	delete(a.sessions, cookie.Value)
	a.mu.Unlock()
}

func (a *authStore) pruneLocked(now time.Time) {
	for token, s := range a.sessions {
		if now.After(s.expires) {
			delete(a.sessions, token)
		}
	}
}

func secureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})
}
