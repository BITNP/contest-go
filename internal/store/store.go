package store

import (
	"context"
	"errors"
	"time"

	"contest-go/internal/model"
)

var (
	ErrDraftNotFound = errors.New("draft not found")
	ErrDraftExists   = errors.New("draft already exists")
	ErrSubmitLocked  = errors.New("submit is locked")
)

// DraftStore 保存答题草稿和自动交卷截止队列。
type DraftStore interface {
	Create(ctx context.Context, p model.Paper, ttl time.Duration) error
	Get(ctx context.Context, username string) (*model.Paper, error)
	SaveAnswer(ctx context.Context, username string, questionID int, choiceIDs []int, ttl time.Duration) error
	Delete(ctx context.Context, username string) error
	Due(ctx context.Context, now time.Time, limit int) ([]string, error)

	LockSubmit(ctx context.Context, username string, ttl time.Duration) (bool, error)
	UnlockSubmit(ctx context.Context, username string) error
}

// ScoreStore 保存最终成绩。
type ScoreStore interface {
	CountAttempts(ctx context.Context, username string) (int, error)
	Insert(ctx context.Context, score model.Score) (bool, error)
	List(ctx context.Context, username string) ([]model.Score, error)
}
