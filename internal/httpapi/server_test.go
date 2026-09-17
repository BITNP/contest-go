package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"contest-go/internal/auth"
	"contest-go/internal/exam"
	"contest-go/internal/model"
	"contest-go/internal/store"
)

func apiTestServer() (*Server, http.Handler) {
	b := model.NewBank()
	b.Questions[1] = model.Question{
		ID:       1,
		Category: model.CategoryRadio,
		Content:  "1+1=?",
		Choices: []model.Choice{
			{ID: 100, Content: "1", Correct: false},
			{ID: 101, Content: "2", Correct: true},
		},
	}
	b.QIDs[model.CategoryRadio] = []int{1}
	b.Questions[2] = model.Question{
		ID:       2,
		Category: model.CategoryMultiple,
		Content:  "选偶数",
		Choices: []model.Choice{
			{ID: 200, Content: "1", Correct: false},
			{ID: 201, Content: "2", Correct: true},
			{ID: 202, Content: "3", Correct: false},
			{ID: 203, Content: "4", Correct: true},
		},
	}
	b.QIDs[model.CategoryMultiple] = []int{2}

	svc := exam.NewService(b, store.NewMemoryDraftStore(), store.NewMemoryScoreStore(), exam.Config{
		PaperCounts: map[model.Category]int{
			model.CategoryRadio:    1,
			model.CategoryMultiple: 1,
		},
		ScorePerQuestion: map[model.Category]int{
			model.CategoryRadio:    5,
			model.CategoryMultiple: 5,
		},
		Deadline: time.Minute,
		MaxTries: 2,
		DraftTTL: time.Hour,
	})
	session := auth.NewSessionManager("test", time.Hour, false)
	srv, err := New(svc, session, nil, true)
	if err != nil {
		panic(err)
	}
	return srv, srv.Handler()
}

func TestAPIFlow(t *testing.T) {
	_, h := apiTestServer()

	dev := httptest.NewRecorder()
	h.ServeHTTP(dev, httptest.NewRequest(http.MethodGet, "/auth/dev?username=u1", nil))
	if dev.Code != http.StatusOK {
		t.Fatalf("dev login status = %d, body=%s", dev.Code, dev.Body.String())
	}
	sessionCookie := cookie(dev.Result().Cookies(), "contest_session")
	csrfCookie := cookie(dev.Result().Cookies(), "contest_csrf")
	if sessionCookie == nil || csrfCookie == nil {
		t.Fatalf("dev login cookies missing: %v", dev.Result().Cookies())
	}

	me := httptest.NewRecorder()
	meReq := httptest.NewRequest(http.MethodGet, "/api/me", nil)
	meReq.AddCookie(sessionCookie)
	h.ServeHTTP(me, meReq)
	if me.Code != http.StatusOK {
		t.Fatalf("GET /api/me = %d, body=%s", me.Code, me.Body.String())
	}
	var meBody struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(me.Body.Bytes(), &meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if meBody.Username != "u1" {
		t.Fatalf("username = %q, want u1", meBody.Username)
	}

	getExam := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/exam", nil)
	req.AddCookie(sessionCookie)
	h.ServeHTTP(getExam, req)
	if getExam.Code != http.StatusOK {
		t.Fatalf("GET /api/exam = %d, body=%s", getExam.Code, getExam.Body.String())
	}
	var ev examView
	if err := json.Unmarshal(getExam.Body.Bytes(), &ev); err != nil {
		t.Fatalf("decode exam: %v", err)
	}
	if len(ev.Questions) != 2 || ev.TotalScore != 10 {
		t.Fatalf("exam = %+v", ev)
	}
	for _, q := range ev.Questions {
		if q.Category == string(model.CategoryRadio) {
			postAnswer(t, h, sessionCookie, csrfCookie, answerRequest{QuestionID: q.ID, ChoiceIDs: []int{101}})
		} else {
			postAnswer(t, h, sessionCookie, csrfCookie, answerRequest{QuestionID: q.ID, ChoiceIDs: []int{201, 203}})
		}
	}

	submit := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/submit", nil)
	req.AddCookie(sessionCookie)
	req.AddCookie(csrfCookie)
	req.Header.Set("X-CSRF-Token", csrfCookie.Value)
	h.ServeHTTP(submit, req)
	if submit.Code != http.StatusOK {
		t.Fatalf("submit = %d, body=%s", submit.Code, submit.Body.String())
	}
	var sv scoreView
	if err := json.Unmarshal(submit.Body.Bytes(), &sv); err != nil {
		t.Fatalf("decode score: %v", err)
	}
	if sv.Score != 10 {
		t.Fatalf("score = %+v, want 10", sv)
	}

	scoresRec := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	req.AddCookie(sessionCookie)
	h.ServeHTTP(scoresRec, req)
	if scoresRec.Code != http.StatusOK {
		t.Fatalf("scores = %d, body=%s", scoresRec.Code, scoresRec.Body.String())
	}
	var scoresBody struct {
		MaxScore int `json:"max_score"`
	}
	if err := json.Unmarshal(scoresRec.Body.Bytes(), &scoresBody); err != nil {
		t.Fatalf("decode scores: %v", err)
	}
	if scoresBody.MaxScore != 10 {
		t.Fatalf("max score = %d, want 10", scoresBody.MaxScore)
	}
}

func postAnswer(t *testing.T, h http.Handler, session, csrf *http.Cookie, body answerRequest) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/answer", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(session)
	req.AddCookie(csrf)
	req.Header.Set("X-CSRF-Token", csrf.Value)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("answer status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func cookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestIndexDevLoginVisibility(t *testing.T) {
	srv, h := apiTestServer()

	for _, tc := range []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "disabled", enabled: false, want: false},
		{name: "enabled", enabled: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv.DevLogin = tc.enabled
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET / = %d, body=%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if strings.Contains(body, "{{if") {
				t.Fatal("页面包含未渲染的模板动作")
			}
			if got := strings.Contains(body, "开发登录"); got != tc.want {
				t.Fatalf("开发登录显示 = %v, want %v", got, tc.want)
			}
			if got := strings.Contains(body, `id="dev-form"`); got != tc.want {
				t.Fatalf("dev-form 显示 = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFrontendAssets(t *testing.T) {
	_, h := apiTestServer()

	for _, path := range []string{
		"/static/css/app.css",
		"/static/js/common.js",
		"/static/js/index.js",
		"/static/js/info.js",
		"/static/js/contest.js",
		"/static/img/bit-icon.svg",
		"/static/img/background/1.jpg",
		"/static/img/home.jpg",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d", path, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET %s 返回空内容", path)
		}
	}
}

func TestPagesRender(t *testing.T) {
	_, h := apiTestServer()

	for _, path := range []string{"/", "/contest", "/info"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d, body=%s", path, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if strings.Contains(body, "{{") {
			t.Errorf("GET %s 包含未渲染的模板动作", path)
		}
		if !strings.Contains(body, "北京理工大学") {
			t.Errorf("GET %s 缺少校名", path)
		}
	}
}
