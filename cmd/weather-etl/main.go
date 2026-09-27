package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/jiminhsieh/data-engineer-golang/internal/app"
	"github.com/jiminhsieh/data-engineer-golang/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "weather-etl: %v\n", err)
		os.Exit(1)
	}
}
func run() error {
	cfg, err := config.FromEnvironment(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return app.Run(ctx, cfg)
}
