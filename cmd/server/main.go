package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"contest-go/internal/auth"
	"contest-go/internal/bank"
	"contest-go/internal/cas"
	"contest-go/internal/config"
	"contest-go/internal/exam"
	"contest-go/internal/httpapi"
	"contest-go/internal/store"

	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("服务退出", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	questions, err := bank.Load(cfg.BankPath)
	if err != nil {
		return err
	}

	var drafts store.DraftStore
	var scores store.ScoreStore
	var pg *store.PGScores
	var rdb *redis.Client

	if cfg.UseMemory {
		logger.Warn("使用内存存储，仅限开发/测试")
		drafts = store.NewMemoryDraftStore()
		scores = store.NewMemoryScoreStore()
	} else {
		opt, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return err
		}
		rdb = redis.NewClient(opt)
		if err := rdb.Ping(ctx).Err(); err != nil {
			return err
		}
		defer rdb.Close()
		if err := bank.SyncToRedis(ctx, rdb, questions); err != nil {
			return err
		}
		drafts = store.NewRedisDraftStore(rdb)

		pg, err = store.NewPGScores(ctx, cfg.PostgresURL, store.PGPoolConfig{
			MaxConns:          int32(cfg.PGMaxConns),
			MinConns:          int32(cfg.PGMinConns),
			MaxConnLifetime:   cfg.PGMaxConnLifetime,
			MaxConnIdleTime:   cfg.PGMaxConnIdleTime,
			HealthCheckPeriod: cfg.PGHealthCheckPeriod,
		})
		if err != nil {
			return err
		}
		defer pg.Close()
		scores = pg
	}

	examSvc := exam.NewService(questions, drafts, scores, exam.Config{
		PaperCounts:      cfg.PaperCounts,
		ScorePerQuestion: cfg.ScorePerQuestion,
		Deadline:         cfg.Deadline,
		MaxTries:         cfg.MaxTries,
		DraftTTL:         cfg.DraftTTL,
		OpenAt:           cfg.OpenAt,
		CloseAt:          cfg.CloseAt,
	})

	session := auth.NewSessionManager(cfg.Secret, cfg.DraftTTL, cfg.CookieSecure)
	casClient := cas.New(cfg.CASServerURL, cfg.CASServiceURL)
	api := httpapi.New(examSvc, session, casClient, cfg.DevLogin, cfg.MaxTries)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := examSvc.ProcessDue(ctx, 100)
				if err != nil {
					logger.Error("自动交卷失败", "err", err)
				} else if n > 0 {
					logger.Info("自动交卷完成", "count", n)
				}
			}
		}
	}()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("HTTP 服务启动", "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
