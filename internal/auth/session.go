package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	sessionCookie = "contest_session"
	csrfCookie    = "contest_csrf"
)

type SessionManager struct {
	Secret []byte
	TTL    time.Duration
	Secure bool
}

func NewSessionManager(secret string, ttl time.Duration, secure bool) *SessionManager {
	return &SessionManager{Secret: []byte(secret), TTL: ttl, Secure: secure}
}

// Set 写入签名 Session Cookie，并同时设置 CSRF Cookie。
func (m *SessionManager) Set(w http.ResponseWriter, username string) error {
	exp := time.Now().Add(m.TTL).Unix()
	payload := username + "|" + strconv.FormatInt(exp, 10)
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + m.sign(payload)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(m.TTL.Seconds()),
		HttpOnly: true,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})

	csrf, err := randomToken()
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    csrf,
		Path:     "/",
		MaxAge:   int(m.TTL.Seconds()),
		HttpOnly: false,
		Secure:   m.Secure,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func (m *SessionManager) Clear(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.SetCookie(w, &http.Cookie{Name: csrfCookie, Value: "", Path: "/", MaxAge: -1})
}

// Username 解析并校验 Session Cookie。
func (m *SessionManager) Username(r *http.Request) (string, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return "", false
	}
	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	payload := string(raw)
	if !hmac.Equal([]byte(parts[1]), []byte(m.sign(payload))) {
		return "", false
	}
	idx := strings.LastIndex(payload, "|")
	if idx < 0 {
		return "", false
	}
	exp, err := strconv.ParseInt(payload[idx+1:], 10, 64)
	if err != nil || time.Now().Unix() > exp {
		return "", false
	}
	return payload[:idx], true
}

// CheckCSRF 校验双提交 Cookie。POST/PUT/PATCH/DELETE 必须携带 X-CSRF-Token。
func (m *SessionManager) CheckCSRF(r *http.Request) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	c, err := r.Cookie(csrfCookie)
	if err != nil || c.Value == "" {
		return false
	}
	return hmac.Equal([]byte(c.Value), []byte(r.Header.Get("X-CSRF-Token")))
}

func (m *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, m.Secret)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

var ErrNoSession = errors.New("no session")
