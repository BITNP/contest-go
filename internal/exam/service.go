package exam

import (
	"context"
	"errors"
	"fmt"
	"time"

	"contest-go/internal/model"
	"contest-go/internal/store"
)

var (
	ErrTooManyTries    = errors.New("too many tries")
	ErrNotOpen         = errors.New("quiz not open")
	ErrNoDraft         = errors.New("no draft")
	ErrDeadlinePassed  = errors.New("deadline passed")
	ErrInvalidQuestion = errors.New("invalid question")
	ErrInvalidAnswer   = errors.New("invalid answer")
)

type Config struct {
	PaperCounts      map[model.Category]int
	ScorePerQuestion map[model.Category]int
	Deadline         time.Duration
	MaxTries         int
	DraftTTL         time.Duration

	OpenAt  *time.Time
	CloseAt *time.Time
}

type Service struct {
	Bank   *model.Bank
	Drafts store.DraftStore
	Scores store.ScoreStore
	Config Config
	Now    func() time.Time
}

func NewService(bank *model.Bank, drafts store.DraftStore, scores store.ScoreStore, cfg Config) *Service {
	return &Service{
		Bank:   bank,
		Drafts: drafts,
		Scores: scores,
		Config: cfg,
		Now:    time.Now,
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) isOpen(now time.Time) bool {
	if s.Config.OpenAt != nil && now.Before(*s.Config.OpenAt) {
		return false
	}
	if s.Config.CloseAt != nil && !now.Before(*s.Config.CloseAt) {
		return false
	}
	return true
}

// GetOrCreatePaper 返回已有草稿；没有则发新卷。
func (s *Service) GetOrCreatePaper(ctx context.Context, username string) (*model.Paper, error) {
	now := s.now()
	if !s.isOpen(now) {
		return nil, ErrNotOpen
	}

	if p, err := s.Drafts.Get(ctx, username); err == nil {
		if now.After(p.Deadline) {
			_, _ = s.Submit(ctx, username)
			return nil, ErrDeadlinePassed
		}
		return p, nil
	} else if !errors.Is(err, store.ErrDraftNotFound) {
		return nil, err
	}

	n, err := s.Scores.CountAttempts(ctx, username)
	if err != nil {
		return nil, err
	}
	if n >= s.Config.MaxTries {
		return nil, ErrTooManyTries
	}

	questionIDs, err := s.Bank.PickPaper(s.Config.PaperCounts)
	if err != nil {
		return nil, err
	}
	p := model.Paper{
		Username:    username,
		AttemptNo:   n + 1,
		QuestionIDs: questionIDs,
		StartedAt:   now,
		Deadline:    now.Add(s.Config.Deadline),
		Answers:     map[int][]int{},
	}
	ttl := p.Deadline.Sub(now) + s.Config.DraftTTL
	if err := s.Drafts.Create(ctx, p, ttl); err != nil {
		if errors.Is(err, store.ErrDraftExists) {
			// 并发发卷：返回另一个请求已经创建的草稿。
			return s.Drafts.Get(ctx, username)
		}
		return nil, err
	}
	return p.Clone(), nil
}

// SaveAnswer 校验并写入单题答案。
func (s *Service) SaveAnswer(ctx context.Context, username string, questionID int, choiceIDs []int) error {
	p, err := s.Drafts.Get(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotFound) {
			return ErrNoDraft
		}
		return err
	}
	if s.now().After(p.Deadline) {
		return ErrDeadlinePassed
	}

	q, ok := s.Bank.Question(questionID)
	if !ok || !containsInt(p.QuestionIDs, questionID) {
		return ErrInvalidQuestion
	}
	if len(choiceIDs) == 0 {
		// 允许多选清空。
		if q.Category != model.CategoryMultiple {
			return ErrInvalidAnswer
		}
	} else {
		if q.Category != model.CategoryMultiple && len(choiceIDs) != 1 {
			return ErrInvalidAnswer
		}
		if q.Category == model.CategoryMultiple && len(choiceIDs) < 1 {
			return ErrInvalidAnswer
		}
		seen := map[int]bool{}
		for _, cid := range choiceIDs {
			if seen[cid] {
				return ErrInvalidAnswer
			}
			seen[cid] = true
			if _, ok := q.ChoiceByID(cid); !ok {
				return ErrInvalidAnswer
			}
		}
	}

	ttl := p.Deadline.Sub(s.now()) + s.Config.DraftTTL
	return s.Drafts.SaveAnswer(ctx, username, questionID, choiceIDs, ttl)
}

// Submit 交卷并写成绩。幂等：重复调用时若草稿已不存在，返回 ErrNoDraft。
func (s *Service) Submit(ctx context.Context, username string) (model.Score, error) {
	locked, err := s.Drafts.LockSubmit(ctx, username, 10*time.Second)
	if err != nil {
		return model.Score{}, err
	}
	if !locked {
		return model.Score{}, store.ErrSubmitLocked
	}
	defer func() { _ = s.Drafts.UnlockSubmit(ctx, username) }()

	p, err := s.Drafts.Get(ctx, username)
	if err != nil {
		if errors.Is(err, store.ErrDraftNotFound) {
			return model.Score{}, ErrNoDraft
		}
		return model.Score{}, err
	}

	score := s.CalculateScore(p)
	result := model.Score{
		Username:    username,
		AttemptNo:   p.AttemptNo,
		Score:       score,
		SubmittedAt: s.now(),
	}
	if _, err := s.Scores.Insert(ctx, result); err != nil {
		return model.Score{}, err
	}
	if err := s.Drafts.Delete(ctx, username); err != nil {
		return model.Score{}, err
	}
	return result, nil
}

// History 返回某个学生的历史成绩。
func (s *Service) History(ctx context.Context, username string) ([]model.Score, error) {
	return s.Scores.List(ctx, username)
}

// ProcessDue 处理到期的自动交卷，返回尝试处理的草稿数。
func (s *Service) ProcessDue(ctx context.Context, limit int) (int, error) {
	usernames, err := s.Drafts.Due(ctx, s.now(), limit)
	if err != nil {
		return 0, err
	}
	processed := 0
	for _, username := range usernames {
		if _, err := s.Submit(ctx, username); err != nil {
			if errors.Is(err, ErrNoDraft) || errors.Is(err, store.ErrSubmitLocked) {
				continue
			}
			return processed, fmt.Errorf("自动交卷 %s: %w", username, err)
		}
		processed++
	}
	return processed, nil
}

func containsInt(xs []int, x int) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
