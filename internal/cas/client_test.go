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

func TestLogoutURL(t *testing.T) {
	// 显式配置优先
	c := New("https://cas.example", "https://contest.example/callback")
	c.LogoutServiceURL = "https://contest.example/bye"
	got := c.LogoutURL()
	want := "https://cas.example/logout?service=https%3A%2F%2Fcontest.example%2Fbye"
	if got != want {
		t.Fatalf("显式 LogoutServiceURL: got %q, want %q", got, want)
	}

	// 留空时回跳到站点首页（从 ServiceURL 推导），而不是 callback
	c = New("https://cas.example", "https://contest.example/auth/cas/callback")
	got = c.LogoutURL()
	want = "https://cas.example/logout?service=https%3A%2F%2Fcontest.example%2F"
	if got != want {
		t.Fatalf("默认回跳站点首页: got %q, want %q", got, want)
	}

	// CAS 未配置时返回空串
	c = New("", "https://contest.example/callback")
	if got := c.LogoutURL(); got != "" {
		t.Fatalf("无 ServerURL 应为空串: got %q", got)
	}

	// ServiceURL 无法推导站点时省略 service 参数
	c = New("https://cas.example", "")
	if got := c.LogoutURL(); got != "https://cas.example/logout?" {
		t.Fatalf("无法推导站点时不带 service: got %q", got)
	}
}
