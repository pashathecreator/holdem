package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/pashathecreator/holdem/services/wallet-service/internal/application"
	"github.com/pashathecreator/holdem/services/wallet-service/internal/auth"
	"github.com/pashathecreator/holdem/services/wallet-service/internal/chain"
	"github.com/pashathecreator/holdem/services/wallet-service/internal/config"
	deliveryhttp "github.com/pashathecreator/holdem/services/wallet-service/internal/delivery/http"
	repopostgres "github.com/pashathecreator/holdem/services/wallet-service/internal/infrastructure/repository/postgres"
	authkafka "github.com/pashathecreator/holdem/services/wallet-service/internal/kafka"
)

const httpShutdownTimeout = 5 * time.Second

type app struct {
	cfg           *config.Config
	httpServer    *http.Server
	metricsServer *http.Server
	pool          *pgxpool.Pool
	consumer      *authkafka.Consumer
	chainClient   *chain.Client
	chainRunner   *chain.Runner
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

	repo := repopostgres.NewRepository(pool)
	chainClient, err := chain.NewClient(cfg.Chain.RPCURL, cfg.Chain.HotWalletAddress, cfg.Chain.HotWalletPrivateKey, cfg.Chain.Enabled)
	if err != nil {
		pool.Close()
		return nil, err
	}

	service := application.NewService(repo, cfg.Chain.Network, chainClient)
	validator := auth.NewValidator(cfg.Auth.Issuer, cfg.Auth.JWKSURL)
	server := deliveryhttp.NewServer(service, validator, cfg.Internal.Token)
	consumer := authkafka.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.UserCreatedTopic, cfg.Kafka.ConsumerGroup, service)
	chainRunner := chain.NewRunner(repo, chainClient, chain.RunnerConfig{
		Confirmations:       cfg.Chain.Confirmations,
		StartBlock:          cfg.Chain.StartBlock,
		ScanInterval:        cfg.Chain.ScanInterval,
		ReceiptPollInterval: cfg.Chain.ReceiptPollInterval,
		HotWalletAddress:    chainClient.HotWalletAddress(),
	}, zap.NewNop())

	return &app{
		cfg:           cfg,
		httpServer:    buildHTTPServer(cfg.HTTP.Addr, server),
		metricsServer: buildMetricsServer(cfg.Metrics.Addr),
		pool:          pool,
		consumer:      consumer,
		chainClient:   chainClient,
		chainRunner:   chainRunner,
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

func (a *app) runConsumer(ctx context.Context, logger *zap.Logger) {
	logger.Info("wallet kafka consumer started", zap.Strings("brokers", a.cfg.Kafka.Brokers), zap.String("topic", a.cfg.Kafka.UserCreatedTopic))
	if err := a.consumer.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Fatal("wallet kafka consumer failed", zap.Error(err))
	}
}

func (a *app) runChain(ctx context.Context, logger *zap.Logger) {
	if a.chainRunner == nil || a.chainClient == nil || !a.chainClient.Enabled() {
		return
	}
	logger.Info("wallet chain runner started", zap.String("network", a.cfg.Chain.Network), zap.Int64("confirmations", a.cfg.Chain.Confirmations))
	a.chainRunner.Run(ctx)
}

func (a *app) shutdown(ctx context.Context, logger *zap.Logger) {
	if err := a.httpServer.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown failed", zap.Error(err))
	}
	if err := a.metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown failed", zap.Error(err))
	}
	if err := a.consumer.Close(); err != nil {
		logger.Error("consumer shutdown failed", zap.Error(err))
	}
	if err := a.chainClient.Close(); err != nil {
		logger.Error("chain client shutdown failed", zap.Error(err))
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
