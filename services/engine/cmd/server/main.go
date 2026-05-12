package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/pashathecreator/holdem/services/engine/internal/config"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() {
		_ = logger.Sync()
	}()

	cfg, err := config.Load("config/config.yaml")
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	app, err := newApp(ctx, cfg)
	if err != nil {
		logger.Fatal("failed to init app", zap.Error(err))
	}

	go app.runGRPC(logger)
	go app.runHTTP(logger)
	go app.runMetrics(logger)

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	app.shutdown(shutdownCtx, logger)

	logger.Info("shutdown complete")
}
