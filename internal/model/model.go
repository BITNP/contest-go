package model

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"time"
)

// Category 是题型。
type Category string

const (
	CategoryBinary   Category = "B" // 判断
	CategoryRadio    Category = "R" // 单选
	CategoryMultiple Category = "M" // 多选
)

func (c Category) Valid() bool {
	switch c {
	case CategoryBinary, CategoryRadio, CategoryMultiple:
		return true
	default:
		return false
	}
}

// Label 返回题型的中文名。
func (c Category) Label() string {
	switch c {
	case CategoryBinary:
		return "判断"
	case CategoryRadio:
		return "单选"
	case CategoryMultiple:
		return "多选"
	default:
		return string(c)
	}
}

// Choice 是题目选项。Correct 只允许在服务端使用，禁止下发给浏览器。
type Choice struct {
	ID      int    `json:"id"`
	Content string `json:"content"`
	Correct bool   `json:"correct"`
}

// Question 是一道题。
type Question struct {
	ID       int      `json:"id"`
	Category Category `json:"category"`
	Content  string   `json:"content"`
	Choices  []Choice `json:"choices"`
}

// CorrectChoiceIDs 返回正确选项 ID。
func (q Question) CorrectChoiceIDs() []int {
	out := make([]int, 0, len(q.Choices))
	for _, c := range q.Choices {
		if c.Correct {
			out = append(out, c.ID)
		}
	}
	sort.Ints(out)
	return out
}

// ChoiceByID 查找选项。
func (q Question) ChoiceByID(id int) (Choice, bool) {
	for _, c := range q.Choices {
		if c.ID == id {
			return c, true
		}
	}
	return Choice{}, false
}

// Bank 是内存中的题库。
type Bank struct {
	Questions map[int]Question
	QIDs      map[Category][]int
}

// NewBank 创建空题库。
func NewBank() *Bank {
	return &Bank{
		Questions: make(map[int]Question),
		QIDs:      make(map[Category][]int),
	}
}

// Question 按 ID 取题。
func (b *Bank) Question(id int) (Question, bool) {
	q, ok := b.Questions[id]
	return q, ok
}

// PickPaper 按题型配置不放回抽题，并打乱最终顺序。
func (b *Bank) PickPaper(counts map[Category]int) ([]int, error) {
	total := 0
	for _, n := range counts {
		if n < 0 {
			return nil, fmt.Errorf("题数不能为负数")
		}
		total += n
	}

	out := make([]int, 0, total)
	for cat, n := range counts {
		if n == 0 {
			continue
		}
		ids := b.QIDs[cat]
		if len(ids) < n {
			return nil, fmt.Errorf("题型 %s 题量不足：需要 %d，题库只有 %d", cat, n, len(ids))
		}
		for _, idx := range rand.Perm(len(ids))[:n] {
			out = append(out, ids[idx])
		}
	}

	rand.Shuffle(len(out), func(i, j int) {
		out[i], out[j] = out[j], out[i]
	})
	return out, nil
}

// Paper 是一份答题草稿。
type Paper struct {
	Username    string        `json:"username"`
	AttemptNo   int           `json:"attempt_no"`
	QuestionIDs []int         `json:"question_ids"`
	StartedAt   time.Time     `json:"started_at"`
	Deadline    time.Time     `json:"deadline"`
	Answers     map[int][]int `json:"answers,omitempty"` // question_id -> choice_ids
}

// Clone 深复制草稿，避免调用方修改 store 内部状态。
func (p *Paper) Clone() *Paper {
	if p == nil {
		return nil
	}
	cp := *p
	cp.QuestionIDs = append([]int(nil), p.QuestionIDs...)
	cp.Answers = make(map[int][]int, len(p.Answers))
	for qid, ids := range p.Answers {
		cp.Answers[qid] = append([]int(nil), ids...)
	}
	return &cp
}

// Score 是一次已提交的最终成绩。
type Score struct {
	Username    string    `json:"username"`
	AttemptNo   int       `json:"attempt_no"`
	Score       int       `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// MaxScore 返回历史最高分。
func MaxScore(scores []Score) int {
	max := 0
	for _, s := range scores {
		if s.Score > max {
			max = s.Score
		}
	}
	return max
}
