package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"contest-go/internal/cas"
)

// 登录并取回试卷，返回会话 Cookie 与试卷响应体。
func loginAndGetExam(t *testing.T, h http.Handler) (*http.Cookie, map[string]any) {
	t.Helper()

	dev := httptest.NewRecorder()
	h.ServeHTTP(dev, httptest.NewRequest(http.MethodGet, "/auth/dev?username=u-clock", nil))
	sessionCookie := cookie(dev.Result().Cookies(), "contest_session")
	if sessionCookie == nil {
		t.Fatal("dev login 未返回会话 Cookie")
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/exam", nil)
	req.AddCookie(sessionCookie)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/exam = %d, body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode exam: %v", err)
	}
	return sessionCookie, body
}

func TestExamProvidesServerClock(t *testing.T) {
	_, h := apiTestServer()
	before := time.Now().UnixMilli()
	_, body := loginAndGetExam(t, h)
	after := time.Now().UnixMilli()

	nowMS, ok := body["now_unix_ms"].(float64)
	if !ok || nowMS == 0 {
		t.Fatalf("now_unix_ms 缺失: %v", body["now_unix_ms"])
	}
	if int64(nowMS) < before || int64(nowMS) > after {
		t.Fatalf("now_unix_ms = %v，不在 [%d, %d] 内", nowMS, before, after)
	}
	deadlineMS, ok := body["deadline_unix_ms"].(float64)
	if !ok || int64(deadlineMS) <= int64(nowMS) {
		t.Fatalf("deadline_unix_ms = %v，应大于 now_unix_ms = %v", deadlineMS, nowMS)
	}
	if _, ok := body["deadline"].(string); !ok {
		t.Fatal("deadline 字段缺失")
	}
}

func TestStaticETag(t *testing.T) {
	_, h := apiTestServer()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/static/js/common.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/js/common.js = %d", rec.Code)
	}
	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("缺少 ETag")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("Content-Type = %q，应为 text/javascript", ct)
	}

	req := httptest.NewRequest(http.MethodGet, "/static/js/common.js", nil)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotModified {
		t.Fatalf("条件请求 = %d，应为 304", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Fatal("304 响应不应携带 body")
	}
}

func TestStaticSecurityHeaders(t *testing.T) {
	_, h := apiTestServer()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "same-origin",
	} {
		if got := rec.Header().Get(name); got != want {
			t.Errorf("%s = %q，应为 %q", name, got, want)
		}
	}
}

func TestLogoutReturnsCASLogoutURL(t *testing.T) {
	srv, _ := New(nil, nil, cas.New("https://cas.example", "https://contest.example/callback"), false, 2)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/auth/dev?username=x", nil))

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /auth/logout = %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode logout: %v", err)
	}
	if !strings.HasPrefix(body["logout_url"], "https://cas.example/logout?") {
		t.Fatalf("logout_url = %q", body["logout_url"])
	}
}

func TestPageNavItems(t *testing.T) {
	srv, h := apiTestServer()
	_ = srv

	// 未登录：只有主页
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body := rec.Body.String()
	if strings.Count(body, `aria-current="page"`) != 2 { // 桌面 + 移动端各一
		t.Fatalf("未登录时 aria-current 出现次数 = %d", strings.Count(body, `aria-current="page"`))
	}
	if strings.Contains(body, ">答题</a>") {
		t.Fatal("未登录不应显示答题导航")
	}

	// 登录：显示答题与历史成绩
	dev := httptest.NewRecorder()
	h.ServeHTTP(dev, httptest.NewRequest(http.MethodGet, "/auth/dev?username=u-nav", nil))
	sessionCookie := cookie(dev.Result().Cookies(), "contest_session")
	req := httptest.NewRequest(http.MethodGet, "/contest", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body = rec.Body.String()
	if !strings.Contains(body, ">答题</a>") || !strings.Contains(body, ">历史成绩</a>") {
		t.Fatal("登录后应显示答题与历史成绩导航")
	}
}

func TestContestBackgroundImage(t *testing.T) {
	_, h := apiTestServer()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/contest", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /contest = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "background-image: url('/static/img/background/") {
		t.Fatal("答题页缺少背景图")
	}
	// 主页不使用背景图
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if strings.Contains(rec.Body.String(), "img/background/") {
		t.Fatal("主页不应引用背景图")
	}
}
