package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := newApp(ctx)
	if err != nil {
		logger.Fatal("init app", zap.Error(err))
	}

	go app.runHTTP(logger)
	go app.runMetrics(logger)

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()

	app.shutdown(shutdownCtx, logger)
}
