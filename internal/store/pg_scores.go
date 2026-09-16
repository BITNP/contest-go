package store

import (
	"context"
	"fmt"
	"time"

	"contest-go/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PGScores struct {
	pool *pgxpool.Pool
}

type PGPoolConfig struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func NewPGScores(ctx context.Context, dsn string, cfg PGPoolConfig) (*PGScores, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析 PostgreSQL DSN: %w", err)
	}
	if cfg.MaxConns > 0 {
		poolCfg.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns >= 0 {
		poolCfg.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("连接 PostgreSQL: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	s := &PGScores{pool: pool}
	if err := s.EnsureSchema(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *PGScores) Close() {
	s.pool.Close()
}

func (s *PGScores) EnsureSchema(ctx context.Context) error {
	const tableDDL = `
CREATE TABLE IF NOT EXISTS scores (
    username      text        NOT NULL,
    attempt_no    smallint    NOT NULL,
    score         smallint    NOT NULL,
    submitted_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (username, attempt_no)
)`
	if _, err := s.pool.Exec(ctx, tableDDL); err != nil {
		return err
	}
	const indexDDL = `
CREATE INDEX IF NOT EXISTS scores_username_time_idx
    ON scores (username, submitted_at DESC)`
	_, err := s.pool.Exec(ctx, indexDDL)
	return err
}

func (s *PGScores) CountAttempts(ctx context.Context, username string) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM scores WHERE username = $1`, username).Scan(&n)
	return n, err
}

func (s *PGScores) Insert(ctx context.Context, score model.Score) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
INSERT INTO scores (username, attempt_no, score, submitted_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (username, attempt_no) DO NOTHING
`, score.Username, score.AttemptNo, score.Score, score.SubmittedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s *PGScores) List(ctx context.Context, username string) ([]model.Score, error) {
	rows, err := s.pool.Query(ctx, `
SELECT username, attempt_no, score, submitted_at
FROM scores
WHERE username = $1
ORDER BY attempt_no
`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Score
	for rows.Next() {
		var sc model.Score
		if err := rows.Scan(&sc.Username, &sc.AttemptNo, &sc.Score, &sc.SubmittedAt); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

var _ ScoreStore = (*PGScores)(nil)
