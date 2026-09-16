package cas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/serviceValidate" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("ticket") != "ST-1" {
			http.Error(w, "bad ticket", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(`<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationSuccess>
    <cas:user>2026001</cas:user>
  </cas:authenticationSuccess>
</cas:serviceResponse>`))
	}))
	defer srv.Close()

	c := New(srv.URL, "https://contest.example/callback")
	username, err := c.Validate(context.Background(), "ST-1")
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if username != "2026001" {
		t.Fatalf("username = %q", username)
	}
	if !strings.HasPrefix(c.LoginURL(), srv.URL+"/login?") {
		t.Fatalf("LoginURL = %q", c.LoginURL())
	}
}

func TestValidateFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<cas:serviceResponse xmlns:cas="http://www.yale.edu/tp/cas">
  <cas:authenticationFailure code="INVALID_TICKET">ticket 无效</cas:authenticationFailure>
</cas:serviceResponse>`))
	}))
	defer srv.Close()

	c := New(srv.URL, "https://contest.example/callback")
	if _, err := c.Validate(context.Background(), "bad"); err == nil {
		t.Fatal("Validate 应该失败")
	}
}
