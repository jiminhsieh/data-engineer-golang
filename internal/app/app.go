package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jiminhsieh/data-engineer-golang/internal/config"
	"github.com/jiminhsieh/data-engineer-golang/internal/lake"
	"github.com/jiminhsieh/data-engineer-golang/internal/observability"
	"github.com/jiminhsieh/data-engineer-golang/internal/pipeline"
	store "github.com/jiminhsieh/data-engineer-golang/internal/postgres"
	"github.com/jiminhsieh/data-engineer-golang/internal/scheduler"
	"github.com/jiminhsieh/data-engineer-golang/internal/transform"
	"github.com/jiminhsieh/data-engineer-golang/internal/weather"
)

type database interface {
	store.DB
	Close()
}

type connectFunc func(context.Context, string) (database, error)

func Run(ctx context.Context, cfg config.Config) error {
	return run(ctx, cfg, func(ctx context.Context, databaseURL string) (database, error) {
		return store.Connect(ctx, databaseURL)
	})
}

func run(ctx context.Context, cfg config.Config, connect connectFunc) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	var mirror *os.File
	if cfg.LogStdout {
		mirror = os.Stdout
	}
	logger, err := observability.OpenLog(cfg.LogFile, mirror)
	if err != nil {
		return err
	}
	defer logger.Close()
	startupCtx, cancel := context.WithTimeout(ctx, cfg.HTTPTimeout)
	defer cancel()
	pool, err := connect(startupCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	repo := store.NewRepository(pool)
	if err := repo.Migrate(startupCtx); err != nil {
		return err
	}
	metrics := observability.NewMetrics()
	client := weather.NewClient(cfg.APIBaseURL, cfg.APIKey, cfg.Latitude, cfg.Longitude, cfg.HTTPTimeout)
	writer := lake.NewWriter(cfg.DataDir, lake.JSONEncoder{})
	worker := pipeline.New(client, repo, writer, transform.Weather, logger.Logger, metrics)
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: observability.Handler(metrics, observability.BoundedPinger{Pinger: repo, Timeout: min(cfg.HTTPTimeout, 2*time.Second)}), ReadHeaderTimeout: 5 * time.Second}
	serverErrors := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()
	schedulerDone := make(chan struct{})
	go func() { defer close(schedulerDone); scheduler.Run(ctx, cfg.Interval, worker.Run, logger.Logger) }()
	logger.Printf("stage=service outcome=started http_addr=%q", cfg.HTTPAddr)
	select {
	case <-ctx.Done():
	case err := <-serverErrors:
		return fmt.Errorf("HTTP server: %w", err)
	}
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	select {
	case <-schedulerDone:
	case <-shutdownCtx.Done():
		return errors.New("pipeline shutdown timed out")
	}
	logger.Print("stage=service outcome=stopped")
	return nil
}
