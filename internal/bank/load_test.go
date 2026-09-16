package bank

import (
	"os"
	"path/filepath"
	"testing"

	"contest-go/internal/model"
)

func TestLoadSimpleFixture(t *testing.T) {
	const fixture = `
- model: quiz.question
  pk: 1
  fields:
    category: R
    content: 单选题
- model: quiz.choice
  fields:
    content: A
    correct: false
    question: 1
- model: quiz.choice
  fields:
    content: B
    correct: true
    question: 1
- model: quiz.choice
  fields:
    content: C
    correct: false
    question: 1

- model: quiz.question
  pk: 2
  fields:
    category: B
    content: 判断题
- model: quiz.choice
  fields:
    content: 正确
    correct: true
    question: 2
- model: quiz.choice
  fields:
    content: 错误
    correct: false
    question: 2

- model: quiz.question
  pk: 3
  fields:
    category: M
    content: 多选题
- model: quiz.choice
  fields:
    content: A
    correct: true
    question: 3
- model: quiz.choice
  fields:
    content: B
    correct: true
    question: 3
- model: quiz.choice
  fields:
    content: C
    correct: false
    question: 3
- model: quiz.choice
  fields:
    content: D
    correct: false
    question: 3
`

	path := filepath.Join(t.TempDir(), "fixture.yaml")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := len(b.Questions), 3; got != want {
		t.Fatalf("题目数 = %d, want %d", got, want)
	}

	wantCounts := map[model.Category]int{
		model.CategoryBinary:   1,
		model.CategoryRadio:    1,
		model.CategoryMultiple: 1,
	}
	for cat, want := range wantCounts {
		if got := len(b.QIDs[cat]); got != want {
			t.Errorf("%s 题数 = %d, want %d", cat, got, want)
		}
	}

	type wantChoice struct {
		id      int
		correct bool
	}
	wantChoices := map[int][]wantChoice{
		1: {
			{100, false},
			{101, true},
			{102, false},
		},
		2: {
			{200, true},
			{201, false},
		},
		3: {
			{300, true},
			{301, true},
			{302, false},
			{303, false},
		},
	}
	for qid, wants := range wantChoices {
		q, ok := b.Question(qid)
		if !ok {
			t.Fatalf("题目 %d 不存在", qid)
		}
		if got, want := len(q.Choices), len(wants); got != want {
			t.Fatalf("题目 %d 选项数 = %d, want %d", qid, got, want)
		}
		for i, want := range wants {
			got := q.Choices[i]
			if got.ID != want.id || got.Correct != want.correct {
				t.Errorf("题目 %d 选项 %d = {id:%d correct:%v}, want {id:%d correct:%v}",
					qid, i, got.ID, got.Correct, want.id, want.correct)
			}
		}
	}
}

func TestValidateRejectsBadMultiple(t *testing.T) {
	b := model.NewBank()
	b.Questions[1] = model.Question{
		ID:       1,
		Category: model.CategoryMultiple,
		Content:  "bad",
		Choices: []model.Choice{
			{ID: 100, Content: "a", Correct: true},
			{ID: 101, Content: "b", Correct: false},
		},
	}
	b.QIDs[model.CategoryMultiple] = []int{1}
	if err := Validate(b); err == nil {
		t.Fatal("Validate 应该拒绝只有 1 个正确项的多选题")
	}
}
