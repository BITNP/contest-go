package store

import (
	"context"
	"sort"
	"sync"
	"time"

	"contest-go/internal/model"
)

// MemoryDraftStore 是测试和本地开发用的内存实现。
type MemoryDraftStore struct {
	mu        sync.Mutex
	papers    map[string]model.Paper
	deadlines map[string]time.Time
	locks     map[string]time.Time
}

func NewMemoryDraftStore() *MemoryDraftStore {
	return &MemoryDraftStore{
		papers:    make(map[string]model.Paper),
		deadlines: make(map[string]time.Time),
		locks:     make(map[string]time.Time),
	}
}

func (s *MemoryDraftStore) Create(_ context.Context, p model.Paper, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.papers[p.Username]; ok {
		return ErrDraftExists
	}
	cp := p.Clone()
	s.papers[p.Username] = *cp
	s.deadlines[p.Username] = cp.Deadline
	return nil
}

func (s *MemoryDraftStore) Get(_ context.Context, username string) (*model.Paper, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.papers[username]
	if !ok {
		return nil, ErrDraftNotFound
	}
	return p.Clone(), nil
}

func (s *MemoryDraftStore) SaveAnswer(_ context.Context, username string, questionID int, choiceIDs []int, _ time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.papers[username]
	if !ok {
		return ErrDraftNotFound
	}
	if p.Answers == nil {
		p.Answers = make(map[int][]int)
	}
	p.Answers[questionID] = append([]int(nil), choiceIDs...)
	s.papers[username] = p
	return nil
}

func (s *MemoryDraftStore) Delete(_ context.Context, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.papers, username)
	delete(s.deadlines, username)
	return nil
}

func (s *MemoryDraftStore) Due(_ context.Context, now time.Time, limit int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, limit)
	for username, deadline := range s.deadlines {
		if !deadline.After(now) {
			out = append(out, username)
			if len(out) >= limit {
				break
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return s.deadlines[out[i]].Before(s.deadlines[out[j]])
	})
	return out, nil
}

func (s *MemoryDraftStore) LockSubmit(_ context.Context, username string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	if exp, ok := s.locks[username]; ok && exp.After(now) {
		return false, nil
	}
	s.locks[username] = now.Add(ttl)
	return true, nil
}

func (s *MemoryDraftStore) UnlockSubmit(_ context.Context, username string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.locks, username)
	return nil
}

// MemoryScoreStore 是测试和本地开发用的内存实现。
type MemoryScoreStore struct {
	mu     sync.Mutex
	scores map[string][]model.Score
}

func NewMemoryScoreStore() *MemoryScoreStore {
	return &MemoryScoreStore{scores: make(map[string][]model.Score)}
}

func (s *MemoryScoreStore) CountAttempts(_ context.Context, username string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.scores[username]), nil
}

func (s *MemoryScoreStore) Insert(_ context.Context, score model.Score) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.scores[score.Username] {
		if existing.AttemptNo == score.AttemptNo {
			return false, nil
		}
	}
	s.scores[score.Username] = append(s.scores[score.Username], score)
	sort.Slice(s.scores[score.Username], func(i, j int) bool {
		return s.scores[score.Username][i].AttemptNo < s.scores[score.Username][j].AttemptNo
	})
	return true, nil
}

func (s *MemoryScoreStore) List(_ context.Context, username string) ([]model.Score, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]model.Score(nil), s.scores[username]...), nil
}
