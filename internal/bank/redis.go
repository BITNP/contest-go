package bank

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"contest-go/internal/model"

	"github.com/redis/go-redis/v9"
)

const (
	bankHashKey = "exam:questions"
	bankQIDsKey = "exam:qids:"
	bankLoaded  = "exam:bank:loaded"
)

// SyncToRedis 把内存题库写入 Redis，供服务重启后作为持久副本。
// 比赛期间不做增量导入；启动时覆盖写一次即可。
func SyncToRedis(ctx context.Context, rdb *redis.Client, b *model.Bank) error {
	pipe := rdb.TxPipeline()
	pipe.Del(ctx, bankHashKey)
	for _, cat := range []model.Category{model.CategoryBinary, model.CategoryRadio, model.CategoryMultiple} {
		pipe.Del(ctx, bankQIDsKey+string(cat))
	}

	for qid, q := range b.Questions {
		raw, err := json.Marshal(q)
		if err != nil {
			return err
		}
		pipe.HSet(ctx, bankHashKey, strconv.Itoa(qid), string(raw))
	}
	for cat, ids := range b.QIDs {
		for _, id := range ids {
			pipe.SAdd(ctx, bankQIDsKey+string(cat), id)
		}
	}
	pipe.Set(ctx, bankLoaded, time.Now().UTC().Format(time.RFC3339Nano), 0)
	_, err := pipe.Exec(ctx)
	return err
}
