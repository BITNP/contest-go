package exam

import (
	"sort"

	"contest-go/internal/model"
)

// CalculateScore 计算一份草稿的总分。
// 多选采用全对得分：学生选择集合与正确集合完全一致才计分。
func (s *Service) CalculateScore(p *model.Paper) int {
	total := 0
	for _, qid := range p.QuestionIDs {
		q, ok := s.Bank.Question(qid)
		if !ok {
			continue
		}
		if sameChoiceSet(p.Answers[qid], q.CorrectChoiceIDs()) {
			total += s.Config.ScorePerQuestion[q.Category]
		}
	}
	return total
}

func sameChoiceSet(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	if len(got) == 0 {
		return false
	}
	g := append([]int(nil), got...)
	w := append([]int(nil), want...)
	sort.Ints(g)
	sort.Ints(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}
