package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"contest-go/internal/model"
)

// Config 是服务配置。全部可通过环境变量覆盖。
type Config struct {
	Addr string

	RedisURL    string
	PostgresURL string
	UseMemory   bool

	BankPath string
	Secret   string

	PaperCounts      map[model.Category]int
	ScorePerQuestion map[model.Category]int
	Deadline         time.Duration
	MaxTries         int
	DraftTTL         time.Duration

	PGMaxConns          int
	PGMinConns          int
	PGMaxConnLifetime   time.Duration
	PGMaxConnIdleTime   time.Duration
	PGHealthCheckPeriod time.Duration

	OpenAt  *time.Time
	CloseAt *time.Time

	CASServerURL  string
	CASServiceURL string
	CASLogoutURL  string
	DevLogin      bool

	CookieSecure bool
}

func Default() Config {
	return Config{
		Addr:             ":8080",
		RedisURL:         "redis://127.0.0.1:6379/0",
		PostgresURL:      "postgres://contest:contest@localhost:5432/contest?sslmode=disable",
		BankPath:         "data/problems.yaml",
		PaperCounts:      map[model.Category]int{model.CategoryBinary: 5, model.CategoryRadio: 10, model.CategoryMultiple: 5},
		ScorePerQuestion: map[model.Category]int{model.CategoryBinary: 5, model.CategoryRadio: 5, model.CategoryMultiple: 5},
		Deadline:         5 * time.Minute,
		MaxTries:         2,
		DraftTTL:         24 * time.Hour,

		PGMaxConns:          50,
		PGMinConns:          5,
		PGMaxConnLifetime:   30 * time.Minute,
		PGMaxConnIdleTime:   5 * time.Minute,
		PGHealthCheckPeriod: time.Minute,

		CookieSecure: false,
		DevLogin:     false,
	}
}

func FromEnv() (Config, error) {
	c := Default()
	c.Addr = env("ADDR", c.Addr)
	c.RedisURL = env("REDIS_URL", c.RedisURL)
	c.PostgresURL = env("POSTGRES_URL", c.PostgresURL)
	c.UseMemory = envBool("USE_MEMORY_STORE", c.UseMemory)
	c.BankPath = env("BANK_PATH", c.BankPath)
	c.Secret = env("SESSION_SECRET", "")
	c.Deadline = envDuration("QUIZ_DEADLINE", c.Deadline)
	c.MaxTries = envInt("QUIZ_MAX_TRIES", c.MaxTries)
	c.DraftTTL = envDuration("QUIZ_DRAFT_TTL", c.DraftTTL)

	c.PGMaxConns = envInt("PG_MAX_CONNS", c.PGMaxConns)
	c.PGMinConns = envInt("PG_MIN_CONNS", c.PGMinConns)
	c.PGMaxConnLifetime = envDuration("PG_MAX_CONN_LIFETIME", c.PGMaxConnLifetime)
	c.PGMaxConnIdleTime = envDuration("PG_MAX_CONN_IDLE_TIME", c.PGMaxConnIdleTime)
	c.PGHealthCheckPeriod = envDuration("PG_HEALTH_CHECK_PERIOD", c.PGHealthCheckPeriod)
	c.PaperCounts[model.CategoryBinary] = envInt("N_B", c.PaperCounts[model.CategoryBinary])
	c.PaperCounts[model.CategoryRadio] = envInt("N_R", c.PaperCounts[model.CategoryRadio])
	c.PaperCounts[model.CategoryMultiple] = envInt("N_M", c.PaperCounts[model.CategoryMultiple])
	c.ScorePerQuestion[model.CategoryBinary] = envInt("SCORE_B", c.ScorePerQuestion[model.CategoryBinary])
	c.ScorePerQuestion[model.CategoryRadio] = envInt("SCORE_R", c.ScorePerQuestion[model.CategoryRadio])
	c.ScorePerQuestion[model.CategoryMultiple] = envInt("SCORE_M", c.ScorePerQuestion[model.CategoryMultiple])

	c.CASServerURL = env("CAS_SERVER_URL", c.CASServerURL)
	c.CASServiceURL = env("CAS_SERVICE_URL", c.CASServiceURL)
	c.CASLogoutURL = env("CAS_LOGOUT_URL", c.CASLogoutURL)
	c.DevLogin = envBool("DEV_LOGIN_ENABLED", c.DevLogin)
	c.CookieSecure = envBool("COOKIE_SECURE", c.CookieSecure)

	if v := os.Getenv("OPEN_AT"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return c, fmt.Errorf("OPEN_AT: %w", err)
		}
		c.OpenAt = &t
	}
	if v := os.Getenv("CLOSE_AT"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return c, fmt.Errorf("CLOSE_AT: %w", err)
		}
		c.CloseAt = &t
	}
	if c.Secret == "" {
		c.Secret = "dev-secret-change-me"
	}
	if c.MaxTries <= 0 {
		return c, fmt.Errorf("QUIZ_MAX_TRIES 必须大于 0")
	}
	if c.Deadline <= 0 {
		return c, fmt.Errorf("QUIZ_DEADLINE 必须大于 0")
	}
	return c, nil
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
