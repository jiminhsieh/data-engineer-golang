package scheduler

import (
	"context"
	"log"
	"time"
)

type RunFunc func(context.Context) error

func Run(ctx context.Context, interval time.Duration, run RunFunc, logger *log.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	RunTicks(ctx, ticker.C, run, logger)
}
func RunTicks(ctx context.Context, ticks <-chan time.Time, run RunFunc, logger *log.Logger) {
	if ctx.Err() != nil {
		return
	}
	if err := run(ctx); err != nil {
		logger.Print("stage=pipeline outcome=failure error=\"run failed\"")
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			if ctx.Err() != nil {
				return
			}
			if err := run(ctx); err != nil {
				logger.Print("stage=pipeline outcome=failure error=\"run failed\"")
			}
		}
	}
}
