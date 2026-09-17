package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"contest-go/internal/auth"
	"contest-go/internal/cas"
	"contest-go/internal/exam"
	"contest-go/internal/model"
)

type Server struct {
	Exam     *exam.Service
	Session  *auth.SessionManager
	CAS      *cas.Client
	DevLogin bool

	templates   pageTemplates
	static      staticAssets
	backgrounds []string
}

type ctxKey string

const userKey ctxKey = "user"

// New 构建 Server。页面模板与静态资源在启动时解析、索引，失败立即返回错误。
func New(examSvc *exam.Service, session *auth.SessionManager, casClient *cas.Client, devLogin bool) (*Server, error) {
	templates, err := parsePageTemplates()
	if err != nil {
		return nil, err
	}
	static, err := loadStaticAssets()
	if err != nil {
		return nil, err
	}
	return &Server{
		Exam:        examSvc,
		Session:     session,
		CAS:         casClient,
		DevLogin:    devLogin,
		templates:   templates,
		static:      static,
		backgrounds: collectBackgrounds(static),
	}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /{$}", s.pageHandler("index.html"))
	mux.HandleFunc("GET /contest", s.pageHandler("contest.html"))
	mux.HandleFunc("GET /info", s.pageHandler("info.html"))
	mux.HandleFunc("GET /static/", s.handleStatic)
	mux.HandleFunc("GET /auth/cas/login", s.handleCASLogin)
	mux.HandleFunc("GET /auth/cas/callback", s.handleCASCallback)
	mux.HandleFunc("GET /auth/dev", s.handleDevLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)

	mux.HandleFunc("GET /api/me", s.requireUser(s.handleMe))
	mux.HandleFunc("GET /api/exam", s.requireUser(s.handleGetExam))
	mux.HandleFunc("POST /api/answer", s.requireUser(s.requireCSRF(s.handleAnswer)))
	mux.HandleFunc("POST /api/submit", s.requireUser(s.requireCSRF(s.handleSubmit)))
	mux.HandleFunc("GET /api/scores", s.requireUser(s.handleScores))

	return secureHeaders(mux)
}

// secureHeaders 给所有响应加基础安全头。
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"username": usernameFrom(r)})
}

func (s *Server) handleCASLogin(w http.ResponseWriter, r *http.Request) {
	if s.CAS == nil {
		writeError(w, http.StatusServiceUnavailable, "CAS 未配置")
		return
	}
	redirect := s.CAS.LoginURL()
	if redirect == "" {
		writeError(w, http.StatusServiceUnavailable, "CAS_SERVER_URL 未配置")
		return
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (s *Server) handleCASCallback(w http.ResponseWriter, r *http.Request) {
	if s.CAS == nil {
		writeError(w, http.StatusServiceUnavailable, "CAS 未配置")
		return
	}
	ticket := r.URL.Query().Get("ticket")
	if ticket == "" {
		writeError(w, http.StatusBadRequest, "缺少 ticket")
		return
	}
	username, err := s.CAS.Validate(r.Context(), ticket)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err := s.Session.Set(w, username); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	http.Redirect(w, r, "/contest", http.StatusFound)
}

func (s *Server) handleDevLogin(w http.ResponseWriter, r *http.Request) {
	if !s.DevLogin {
		writeError(w, http.StatusForbidden, "开发登录未启用")
		return
	}
	username := r.URL.Query().Get("username")
	if username == "" {
		writeError(w, http.StatusBadRequest, "缺少 username")
		return
	}
	if err := s.Session.Set(w, username); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"username": username})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.Session.Clear(w)
	if s.CAS != nil {
		if u := s.CAS.LogoutURL(); u != "" {
			writeJSON(w, http.StatusOK, map[string]string{"logout_url": u})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

func (s *Server) requireUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, ok := s.Session.Username(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "请先登录")
			return
		}
		ctx := context.WithValue(r.Context(), userKey, username)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireCSRF(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Session.CheckCSRF(r) {
			writeError(w, http.StatusForbidden, "CSRF 校验失败")
			return
		}
		next(w, r)
	}
}

type choiceView struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
}

type questionView struct {
	ID       int          `json:"id"`
	Category string       `json:"category"`
	Content  string       `json:"content"`
	Choices  []choiceView `json:"choices"`
}

type examView struct {
	AttemptNo       int            `json:"attempt_no"`
	Deadline        time.Time      `json:"deadline"`
	DeadlineUnixMS  int64          `json:"deadline_unix_ms"`
	NowUnixMS       int64          `json:"now_unix_ms"`
	DeadlineSeconds int            `json:"deadline_seconds"`
	Questions       []questionView `json:"questions"`
	Answers         map[int][]int  `json:"answers"`
	TotalScore      int            `json:"total_score"`
}

type answerRequest struct {
	QuestionID int   `json:"question_id"`
	ChoiceIDs  []int `json:"choice_ids"`
}

type scoreView struct {
	AttemptNo   int       `json:"attempt_no"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

func (s *Server) handleGetExam(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r)
	p, err := s.Exam.GetOrCreatePaper(r.Context(), username)
	if err != nil {
		s.writeExamError(w, err)
		return
	}
	view := examView{
		AttemptNo:       p.AttemptNo,
		Deadline:        p.Deadline,
		DeadlineUnixMS:  p.Deadline.UnixMilli(),
		NowUnixMS:       time.Now().UnixMilli(),
		DeadlineSeconds: int(s.Exam.Config.Deadline.Seconds()),
		Answers:         p.Answers,
		TotalScore:      totalScore(s.Exam.Config.ScorePerQuestion, s.Exam.Config.PaperCounts),
		Questions:       make([]questionView, 0, len(p.QuestionIDs)),
	}
	for _, qid := range p.QuestionIDs {
		q, ok := s.Exam.Bank.Question(qid)
		if !ok {
			continue
		}
		qv := questionView{ID: q.ID, Category: string(q.Category), Content: q.Content}
		for _, c := range q.Choices {
			qv.Choices = append(qv.Choices, choiceView{ID: c.ID, Content: c.Content})
		}
		view.Questions = append(view.Questions, qv)
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleAnswer(w http.ResponseWriter, r *http.Request) {
	var req answerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求 JSON 非法")
		return
	}
	username := usernameFrom(r)
	if err := s.Exam.SaveAnswer(r.Context(), username, req.QuestionID, req.ChoiceIDs); err != nil {
		s.writeExamError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r)
	score, err := s.Exam.Submit(r.Context(), username)
	if err != nil {
		if errors.Is(err, exam.ErrNoDraft) {
			// 可能已由自动交卷提交，返回当前历史。
			writeJSON(w, http.StatusOK, map[string]any{"status": "already_submitted"})
			return
		}
		s.writeExamError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, scoreView{
		AttemptNo:   score.AttemptNo,
		Score:       score.Score,
		SubmittedAt: score.SubmittedAt,
	})
}

func (s *Server) handleScores(w http.ResponseWriter, r *http.Request) {
	username := usernameFrom(r)
	scores, err := s.Exam.History(r.Context(), username)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	views := make([]scoreView, 0, len(scores))
	for _, sc := range scores {
		views = append(views, scoreView{AttemptNo: sc.AttemptNo, Score: sc.Score, SubmittedAt: sc.SubmittedAt})
	}
	maxTries := s.Exam.Config.MaxTries
	left := max(maxTries-len(scores), 0)
	writeJSON(w, http.StatusOK, map[string]any{
		"scores":        views,
		"max_score":     model.MaxScore(scores),
		"attempts_left": left,
		"max_tries":     maxTries,
		"total_score":   totalScore(s.Exam.Config.ScorePerQuestion, s.Exam.Config.PaperCounts),
	})
}

func (s *Server) writeExamError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, exam.ErrNotOpen):
		writeError(w, http.StatusForbidden, "答题尚未开放或已结束")
	case errors.Is(err, exam.ErrTooManyTries):
		writeError(w, http.StatusForbidden, "答题次数已用完")
	case errors.Is(err, exam.ErrDeadlinePassed):
		writeError(w, http.StatusForbidden, "本次答题已超时")
	case errors.Is(err, exam.ErrNoDraft):
		writeError(w, http.StatusNotFound, "没有进行中的答卷")
	case errors.Is(err, exam.ErrInvalidQuestion):
		writeError(w, http.StatusBadRequest, "题目不在本卷中")
	case errors.Is(err, exam.ErrInvalidAnswer):
		writeError(w, http.StatusBadRequest, "选项非法")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func usernameFrom(r *http.Request) string {
	if v, ok := r.Context().Value(userKey).(string); ok {
		return v
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func totalScore(per map[model.Category]int, counts map[model.Category]int) int {
	total := 0
	for cat, n := range counts {
		total += per[cat] * n
	}
	return total
}
