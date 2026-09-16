package bank

import (
	"fmt"
	"os"

	"contest-go/internal/model"

	"gopkg.in/yaml.v3"
)

type fixtureRecord struct {
	Model  string `yaml:"model"`
	PK     int    `yaml:"pk"`
	Fields struct {
		Category string `yaml:"category"`
		Content  string `yaml:"content"`
		Correct  bool   `yaml:"correct"`
		Question int    `yaml:"question"`
	} `yaml:"fields"`
}

type choiceRecord struct {
	content string
	correct bool
}

// Load 从 Django fixture 风格的 YAML 读入题库。
// 题目 pk 来自 YAML；选项没有 pk，这里按题目内出现顺序生成 choice_id = question_id*100 + index。
func Load(path string) (*model.Bank, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取题库文件: %w", err)
	}
	var records []fixtureRecord
	if err := yaml.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("解析题库 YAML: %w", err)
	}

	b := model.NewBank()
	choiceRecords := map[int][]choiceRecord{}
	seenQuestion := map[int]bool{}

	for _, rec := range records {
		switch rec.Model {
		case "quiz.question":
			cat := model.Category(rec.Fields.Category)
			if !cat.Valid() {
				return nil, fmt.Errorf("题目 %d 的 category 非法: %q", rec.PK, rec.Fields.Category)
			}
			if rec.Fields.Content == "" {
				return nil, fmt.Errorf("题目 %d 内容为空", rec.PK)
			}
			if seenQuestion[rec.PK] {
				return nil, fmt.Errorf("题目 pk 重复: %d", rec.PK)
			}
			seenQuestion[rec.PK] = true
			b.Questions[rec.PK] = model.Question{
				ID:       rec.PK,
				Category: cat,
				Content:  rec.Fields.Content,
			}
			b.QIDs[cat] = append(b.QIDs[cat], rec.PK)
		case "quiz.choice":
			if _, ok := seenQuestion[rec.Fields.Question]; !ok {
				return nil, fmt.Errorf("选项引用了不存在的题目: %d", rec.Fields.Question)
			}
			choiceRecords[rec.Fields.Question] = append(choiceRecords[rec.Fields.Question], choiceRecord{
				content: rec.Fields.Content,
				correct: rec.Fields.Correct,
			})
		default:
			return nil, fmt.Errorf("未知 model: %q", rec.Model)
		}
	}

	for qid, recs := range choiceRecords {
		q, ok := b.Questions[qid]
		if !ok {
			continue
		}
		choices := make([]model.Choice, 0, len(recs))
		for idx, rec := range recs {
			if rec.content == "" {
				return nil, fmt.Errorf("题目 %d 第 %d 个选项内容为空", qid, idx)
			}
			choices = append(choices, model.Choice{
				ID:      qid*100 + idx,
				Content: rec.content,
				Correct: rec.correct,
			})
		}
		q.Choices = choices
		b.Questions[qid] = q
	}

	if err := Validate(b); err != nil {
		return nil, err
	}
	return b, nil
}

// Validate 校验题库是否满足题型约束。
func Validate(b *model.Bank) error {
	if len(b.Questions) == 0 {
		return fmt.Errorf("题库为空")
	}
	for qid, q := range b.Questions {
		if len(q.Choices) < 2 {
			return fmt.Errorf("题目 %d 选项不足 2 个", qid)
		}
		nCorrect := 0
		seenChoice := map[int]bool{}
		for _, ch := range q.Choices {
			if seenChoice[ch.ID] {
				return fmt.Errorf("题目 %d 选项 ID 重复: %d", qid, ch.ID)
			}
			seenChoice[ch.ID] = true
			if ch.Correct {
				nCorrect++
			}
		}
		switch q.Category {
		case model.CategoryBinary:
			if len(q.Choices) != 2 {
				return fmt.Errorf("判断题 %d 应有 2 个选项，实际 %d 个", qid, len(q.Choices))
			}
			if nCorrect != 1 {
				return fmt.Errorf("判断题 %d 应恰有 1 个正确选项，实际 %d 个", qid, nCorrect)
			}
		case model.CategoryRadio:
			if nCorrect != 1 {
				return fmt.Errorf("单选题 %d 应恰有 1 个正确选项，实际 %d 个", qid, nCorrect)
			}
		case model.CategoryMultiple:
			if nCorrect < 2 {
				return fmt.Errorf("多选题 %d 应至少 2 个正确选项，实际 %d 个", qid, nCorrect)
			}
		default:
			return fmt.Errorf("题目 %d 题型非法: %q", qid, q.Category)
		}
	}
	return nil
}
