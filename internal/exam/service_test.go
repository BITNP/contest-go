package exam

import (
	"context"
	"errors"
	"testing"
	"time"

	"contest-go/internal/model"
	"contest-go/internal/store"
)

func newTestService(now time.Time) (*Service, *store.MemoryDraftStore, *store.MemoryScoreStore) {
	drafts := store.NewMemoryDraftStore()
	scores := store.NewMemoryScoreStore()
	svc := NewService(scoringBank(), drafts, scores, Config{
		PaperCounts: map[model.Category]int{
			model.CategoryBinary:   1,
			model.CategoryRadio:    1,
			model.CategoryMultiple: 1,
		},
		ScorePerQuestion: map[model.Category]int{
			model.CategoryBinary:   5,
			model.CategoryRadio:    5,
			model.CategoryMultiple: 5,
		},
		Deadline: time.Minute,
		MaxTries: 2,
		DraftTTL: time.Hour,
	})
	svc.Now = func() time.Time { return now }
	return svc, drafts, scores
}

func TestExamFlow(t *testing.T) {
	now := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	svc, _, _ := newTestService(now)
	ctx := context.Background()

	p, err := svc.GetOrCreatePaper(ctx, "2026001")
	if err != nil {
		t.Fatalf("GetOrCreatePaper: %v", err)
	}
	if p.AttemptNo != 1 || len(p.QuestionIDs) != 3 {
		t.Fatalf("paper = %+v", p)
	}
	if p.Deadline.Sub(p.StartedAt) != time.Minute {
		t.Fatalf("deadline = %v", p.Deadline.Sub(p.StartedAt))
	}

	if err := svc.SaveAnswer(ctx, "2026001", 1, []int{100}); err != nil {
		t.Fatalf("SaveAnswer B: %v", err)
	}
	if err := svc.SaveAnswer(ctx, "2026001", 2, []int{201}); err != nil {
		t.Fatalf("SaveAnswer R: %v", err)
	}
	if err := svc.SaveAnswer(ctx, "2026001", 3, []int{300, 301, 303}); err != nil {
		t.Fatalf("SaveAnswer M: %v", err)
	}
	if err := svc.SaveAnswer(ctx, "2026001", 2, []int{200, 201}); err == nil {
		t.Fatal("单选提交两个选项应该失败")
	}

	score, err := svc.Submit(ctx, "2026001")
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if score.Score != 15 || score.AttemptNo != 1 {
		t.Fatalf("score = %+v, want 15/attempt1", score)
	}
	if _, err := svc.GetOrCreatePaper(ctx, "2026001"); err != nil {
		t.Fatalf("second attempt: %v", err)
	}
	scores, err := svc.History(ctx, "2026001")
	if err != nil || len(scores) != 1 {
		t.Fatalf("History = %+v, err=%v", scores, err)
	}
}

func TestTooManyTries(t *testing.T) {
	now := time.Now()
	svc, _, scores := newTestService(now)
	ctx := context.Background()
	if _, err := scores.Insert(ctx, model.Score{Username: "u", AttemptNo: 1, Score: 10}); err != nil {
		t.Fatal(err)
	}
	if _, err := scores.Insert(ctx, model.Score{Username: "u", AttemptNo: 2, Score: 20}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.GetOrCreatePaper(ctx, "u")
	if !errors.Is(err, ErrTooManyTries) {
		t.Fatalf("err = %v, want ErrTooManyTries", err)
	}
}

func TestProcessDueSubmitsExpiredDraft(t *testing.T) {
	base := time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
	svc, _, _ := newTestService(base)
	ctx := context.Background()
	if _, err := svc.GetOrCreatePaper(ctx, "u"); err != nil {
		t.Fatal(err)
	}
	svc.Now = func() time.Time { return base.Add(2 * time.Minute) }
	n, err := svc.ProcessDue(ctx, 10)
	if err != nil {
		t.Fatalf("ProcessDue: %v", err)
	}
	if n != 1 {
		t.Fatalf("ProcessDue count = %d, want 1", n)
	}
	scores, err := svc.History(ctx, "u")
	if err != nil || len(scores) != 1 {
		t.Fatalf("History = %+v, err=%v", scores, err)
	}
}
