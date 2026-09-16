package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionAndCSRF(t *testing.T) {
	m := NewSessionManager("test-secret", time.Hour, false)
	rec := httptest.NewRecorder()
	if err := m.Set(rec, "2026001"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	res := rec.Result()
	sessionCookie := findCookie(res.Cookies(), sessionCookie)
	csrfCookie := findCookie(res.Cookies(), csrfCookie)
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatal("Set 没有写入 session/csrf cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(sessionCookie)
	if got, ok := m.Username(req); !ok || got != "2026001" {
		t.Fatalf("Username = %q, %v", got, ok)
	}

	post := httptest.NewRequest(http.MethodPost, "/", nil)
	post.AddCookie(csrfCookie)
	post.Header.Set("X-CSRF-Token", csrfCookie.Value)
	if !m.CheckCSRF(post) {
		t.Fatal("CSRF 校验应该通过")
	}
	post.Header.Set("X-CSRF-Token", "bad")
	if m.CheckCSRF(post) {
		t.Fatal("CSRF 校验应该失败")
	}
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
