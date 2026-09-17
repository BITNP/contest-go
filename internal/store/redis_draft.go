package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"contest-go/internal/model"

	"github.com/redis/go-redis/v9"
)

type RedisDraftStore struct {
	rdb *redis.Client
}

func NewRedisDraftStore(rdb *redis.Client) *RedisDraftStore {
	return &RedisDraftStore{rdb: rdb}
}

func draftKey(username string) string      { return "draft:" + username }
func answersKey(username string) string    { return "draft:" + username + ":answers" }
func deadlinesKey() string                 { return "deadlines" }
func submitLockKey(username string) string { return "submit_lock:" + username }

var createDraftScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 1 then
    return 0
end
redis.call('HSET', KEYS[1],
    'started_at', ARGV[1],
    'deadline', ARGV[2],
    'attempt_no', ARGV[3],
    'question_ids', ARGV[4])
redis.call('DEL', KEYS[2])
redis.call('ZADD', KEYS[3], ARGV[5], ARGV[6])
redis.call('EXPIRE', KEYS[1], ARGV[7])
return 1
`)

func (s *RedisDraftStore) Create(ctx context.Context, p model.Paper, ttl time.Duration) error {
	qids, err := json.Marshal(p.QuestionIDs)
	if err != nil {
		return err
	}
	ok, err := createDraftScript.Run(ctx, s.rdb,
		[]string{draftKey(p.Username), answersKey(p.Username), deadlinesKey()},
		p.StartedAt.Format(time.RFC3339Nano),
		p.Deadline.Format(time.RFC3339Nano),
		p.AttemptNo,
		string(qids),
		p.Deadline.UnixMilli(),
		p.Username,
		int64(ttl.Seconds()),
	).Int()
	if err != nil {
		return err
	}
	if ok == 0 {
		return ErrDraftExists
	}
	return nil
}

func (s *RedisDraftStore) Get(ctx context.Context, username string) (*model.Paper, error) {
	fields, err := s.rdb.HGetAll(ctx, draftKey(username)).Result()
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, ErrDraftNotFound
	}
	started, err := time.Parse(time.RFC3339Nano, fields["started_at"])
	if err != nil {
		return nil, fmt.Errorf("解析 started_at: %w", err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, fields["deadline"])
	if err != nil {
		return nil, fmt.Errorf("解析 deadline: %w", err)
	}
	attempt, err := strconv.Atoi(fields["attempt_no"])
	if err != nil {
		return nil, fmt.Errorf("解析 attempt_no: %w", err)
	}
	var questionIDs []int
	if err := json.Unmarshal([]byte(fields["question_ids"]), &questionIDs); err != nil {
		return nil, fmt.Errorf("解析 question_ids: %w", err)
	}

	answers := map[int][]int{}
	rawAnswers, err := s.rdb.HGetAll(ctx, answersKey(username)).Result()
	if err != nil {
		return nil, err
	}
	for qidStr, raw := range rawAnswers {
		qid, err := strconv.Atoi(qidStr)
		if err != nil {
			continue
		}
		var ids []int
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			continue
		}
		answers[qid] = ids
	}
	return &model.Paper{
		Username:    username,
		AttemptNo:   attempt,
		QuestionIDs: questionIDs,
		StartedAt:   started,
		Deadline:    deadline,
		Answers:     answers,
	}, nil
}

func (s *RedisDraftStore) SaveAnswer(ctx context.Context, username string, questionID int, choiceIDs []int, ttl time.Duration) error {
	raw, err := json.Marshal(choiceIDs)
	if err != nil {
		return err
	}
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, answersKey(username), strconv.Itoa(questionID), string(raw))
	pipe.Expire(ctx, answersKey(username), ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisDraftStore) Delete(ctx context.Context, username string) error {
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, draftKey(username), answersKey(username))
	pipe.ZRem(ctx, deadlinesKey(), username)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisDraftStore) Due(ctx context.Context, now time.Time, limit int) ([]string, error) {
	return s.rdb.ZRangeArgs(ctx, redis.ZRangeArgs{
		Key:     deadlinesKey(),
		Start:   "-inf",
		Stop:    strconv.FormatInt(now.UnixMilli(), 10),
		ByScore: true,
		Offset:  0,
		Count:   int64(limit),
	}).Result()
}

func (s *RedisDraftStore) LockSubmit(ctx context.Context, username string, ttl time.Duration) (bool, error) {
	return s.rdb.SetNX(ctx, submitLockKey(username), "1", ttl).Result()
}

func (s *RedisDraftStore) UnlockSubmit(ctx context.Context, username string) error {
	return s.rdb.Del(ctx, submitLockKey(username)).Err()
}

var _ DraftStore = (*RedisDraftStore)(nil)
