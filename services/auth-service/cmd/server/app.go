package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/pashathecreator/holdem/services/auth-service/internal/application"
	"github.com/pashathecreator/holdem/services/auth-service/internal/config"
	deliveryhttp "github.com/pashathecreator/holdem/services/auth-service/internal/delivery/http"
	repopostgres "github.com/pashathecreator/holdem/services/auth-service/internal/infrastructure/repository/postgres"
	authkafka "github.com/pashathecreator/holdem/services/auth-service/internal/kafka"
	"github.com/pashathecreator/holdem/services/auth-service/internal/security"
)

const httpShutdownTimeout = 5 * time.Second

type app struct {
	cfg           *config.Config
	httpServer    *http.Server
	metricsServer *http.Server
	pool          *pgxpool.Pool
	publisher     *authkafka.Publisher
}

func newApp(ctx context.Context) (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	pool, err := buildPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}

	tokenManager, err := security.NewTokenManager(
		cfg.JWT.Issuer,
		cfg.JWT.KeyID,
		cfg.JWT.PrivateKeyPath,
		cfg.JWT.AccessTTL,
		cfg.JWT.RefreshTTL,
	)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("init token manager: %w", err)
	}

	publisher := authkafka.NewPublisher(cfg.Kafka.Brokers, cfg.Kafka.UserCreatedTopic)
	repo := repopostgres.NewRepository(pool)
	service := application.NewService(repo, tokenManager, publisher, cfg.Admin.Emails)
	server := deliveryhttp.NewServer(service, tokenManager)

	return &app{
		cfg:           cfg,
		httpServer:    buildHTTPServer(cfg.HTTP.Addr, server),
		metricsServer: buildMetricsServer(cfg.Metrics.Addr),
		pool:          pool,
		publisher:     publisher,
	}, nil
}

func (a *app) runHTTP(logger *zap.Logger) {
	logger.Info("http server started", zap.String("addr", a.cfg.HTTP.Addr))
	if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("http server failed", zap.Error(err))
	}
}

func (a *app) runMetrics(logger *zap.Logger) {
	logger.Info("metrics server started", zap.String("addr", a.cfg.Metrics.Addr))
	if err := a.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("metrics server failed", zap.Error(err))
	}
}

func (a *app) shutdown(ctx context.Context, logger *zap.Logger) {
	if err := a.httpServer.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown failed", zap.Error(err))
	}
	if err := a.metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown failed", zap.Error(err))
	}
	if err := a.publisher.Close(); err != nil {
		logger.Error("publisher shutdown failed", zap.Error(err))
	}
	a.pool.Close()
}

func buildPool(ctx context.Context, cfg config.PostgresConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return pool, nil
}

func buildMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &http.Server{Addr: addr, Handler: mux}
}
