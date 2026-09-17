package exam

import (
	"testing"
	"time"

	"contest-go/internal/model"
)

func scoringBank() *model.Bank {
	b := model.NewBank()
	b.Questions[1] = model.Question{
		ID:       1,
		Category: model.CategoryBinary,
		Content:  "判断",
		Choices: []model.Choice{
			{ID: 100, Content: "对", Correct: true},
			{ID: 101, Content: "错", Correct: false},
		},
	}
	b.Questions[2] = model.Question{
		ID:       2,
		Category: model.CategoryRadio,
		Content:  "单选",
		Choices: []model.Choice{
			{ID: 200, Content: "A", Correct: false},
			{ID: 201, Content: "B", Correct: true},
			{ID: 202, Content: "C", Correct: false},
		},
	}
	b.Questions[3] = model.Question{
		ID:       3,
		Category: model.CategoryMultiple,
		Content:  "多选",
		Choices: []model.Choice{
			{ID: 300, Content: "A", Correct: true},
			{ID: 301, Content: "B", Correct: true},
			{ID: 302, Content: "C", Correct: false},
			{ID: 303, Content: "D", Correct: true},
		},
	}
	b.QIDs[model.CategoryBinary] = []int{1}
	b.QIDs[model.CategoryRadio] = []int{2}
	b.QIDs[model.CategoryMultiple] = []int{3}
	return b
}

func scoringService() *Service {
	return NewService(scoringBank(), nil, nil, Config{
		ScorePerQuestion: map[model.Category]int{
			model.CategoryBinary:   5,
			model.CategoryRadio:    5,
			model.CategoryMultiple: 5,
		},
	})
}

func TestCalculateScore(t *testing.T) {
	s := scoringService()
	paper := &model.Paper{
		QuestionIDs: []int{1, 2, 3},
		Answers: map[int][]int{
			1: {100},
			2: {201},
			3: {300, 301, 303},
		},
	}
	if got, want := s.CalculateScore(paper), 15; got != want {
		t.Fatalf("CalculateScore = %d, want %d", got, want)
	}
}

func TestMultipleAllOrNothing(t *testing.T) {
	s := scoringService()
	cases := []struct {
		name string
		ids  []int
		want int
	}{
		{"exact", []int{300, 301, 303}, 5},
		{"missing", []int{300, 301}, 0},
		{"extra", []int{300, 301, 303, 302}, 0},
		{"wrong", []int{300, 302}, 0},
		{"empty", nil, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &model.Paper{
				QuestionIDs: []int{3},
				Answers:     map[int][]int{3: tc.ids},
			}
			if got := s.CalculateScore(p); got != tc.want {
				t.Fatalf("score = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestScoringServiceClockCanBeFixed(t *testing.T) {
	fixed := time.Now()
	s := scoringService()
	s.clock = func() time.Time { return fixed }
	if got := s.clock(); !got.Equal(fixed) {
		t.Fatal("clock 注入失败")
	}
}
