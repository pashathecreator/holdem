package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/pashathecreator/holdem/services/engine/internal/application"
	"github.com/pashathecreator/holdem/services/engine/internal/config"
	deliverygrpc "github.com/pashathecreator/holdem/services/engine/internal/delivery/grpc"
	deliverykafka "github.com/pashathecreator/holdem/services/engine/internal/delivery/kafka"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/evaluator"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/repository/postgres"
	"github.com/pashathecreator/holdem/services/engine/internal/infrastructure/shuffle"
	"github.com/pashathecreator/holdem/services/engine/internal/metrics"
	"github.com/pashathecreator/holdem/services/engine/internal/telemetry"
)

type app struct {
	grpcServer     *grpc.Server
	httpServer     *http.Server
	metricsServer  *http.Server
	pool           *pgxpool.Pool
	publisher      *deliverykafka.Publisher
	tracerShutdown func(context.Context) error
	cfg            *config.Config
}

func newApp(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*app, error) {
	_, tracerShutdown, err := telemetry.New(ctx, cfg.Telemetry.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("app: init telemetry: %w", err)
	}

	pool, err := buildPool(ctx, cfg.Postgres)
	if err != nil {
		return nil, fmt.Errorf("app: build pool: %w", err)
	}

	publisher, err := deliverykafka.NewPublisher(cfg.Kafka.Brokers, cfg.Kafka.SchemaRegistryURL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("app: build publisher: %w", err)
	}

	repo := postgres.NewTracedGameStateRepo(postgres.NewGameStateRepo(pool))
	eval := evaluator.New()
	pubsub := deliverygrpc.NewPubSub()

	rakeConfig := domain.RakeConfig{
		Percent:      cfg.Rake.Percent,
		Cap:          cfg.Rake.Cap,
		NoFlopNoDrop: cfg.Rake.NoFlopNoDrop,
	}

	finishHand := application.NewFinishHand(repo, publisher, eval, rakeConfig)
	startHand := application.NewStartHand(repo, publisher, shuffle.Shuffle)
	applyAction := application.NewApplyAction(repo, publisher, finishHand)

	grpcServer := buildGRPCServer(startHand, applyAction, finishHand, repo, pubsub)
	httpServer := buildHTTPServer(ctx, cfg.GRPC.Addr, cfg.HTTP.Addr)
	metricsServer := buildMetricsServer(cfg.Metrics.Addr)

	go collectPgxPoolMetrics(ctx, pool)

	return &app{
		grpcServer:     grpcServer,
		httpServer:     httpServer,
		metricsServer:  metricsServer,
		pool:           pool,
		publisher:      publisher,
		tracerShutdown: tracerShutdown,
		cfg:            cfg,
	}, nil
}

func (a *app) runGRPC(logger *zap.Logger) {
	lis, err := buildListener(a.cfg.GRPC.Addr)
	if err != nil {
		logger.Fatal("failed to listen grpc", zap.Error(err))
	}
	logger.Info("grpc server started", zap.String("addr", a.cfg.GRPC.Addr))
	if err := a.grpcServer.Serve(lis); err != nil {
		logger.Fatal("grpc server failed", zap.Error(err))
	}
}

func (a *app) runHTTP(logger *zap.Logger) {
	logger.Info("http gateway started", zap.String("addr", a.cfg.HTTP.Addr))
	if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("http gateway failed", zap.Error(err))
	}
}

func (a *app) runMetrics(logger *zap.Logger) {
	logger.Info("metrics server started", zap.String("addr", a.cfg.Metrics.Addr))
	if err := a.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("metrics server failed", zap.Error(err))
	}
}

func (a *app) shutdown(ctx context.Context, logger *zap.Logger) {
	a.grpcServer.GracefulStop()

	if err := a.httpServer.Shutdown(ctx); err != nil {
		logger.Error("http gateway shutdown failed", zap.Error(err))
	}

	if err := a.metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown failed", zap.Error(err))
	}

	if err := a.tracerShutdown(ctx); err != nil {
		logger.Error("tracer shutdown failed", zap.Error(err))
	}

	a.pool.Close()
	a.publisher.Close()
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
	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}

func collectPgxPoolMetrics(ctx context.Context, pool *pgxpool.Pool) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stat := pool.Stat()
			metrics.PgxPoolAcquired.Set(float64(stat.AcquiredConns()))
			metrics.PgxPoolIdle.Set(float64(stat.IdleConns()))
		}
	}
}

var _ trace.TracerProvider = nil
