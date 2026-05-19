package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/pashathecreator/holdem/services/table-manager/internal/application"
	"github.com/pashathecreator/holdem/services/table-manager/internal/config"
	deliverygrpc "github.com/pashathecreator/holdem/services/table-manager/internal/delivery/grpc"
	"github.com/pashathecreator/holdem/services/table-manager/internal/engineclient"
	repopostgres "github.com/pashathecreator/holdem/services/table-manager/internal/infrastructure/repository/postgres"
	"github.com/pashathecreator/holdem/services/table-manager/internal/telemetry"
	"github.com/pashathecreator/holdem/services/table-manager/internal/walletclient"
)

const httpShutdownTimeout = 5 * time.Second

type app struct {
	cfg            *config.Config
	grpcServer     *grpc.Server
	httpServer     *http.Server
	metricsServer  *http.Server
	pool           *pgxpool.Pool
	engine         *engineclient.Client
	tracerShutdown func(context.Context) error
}

func newApp(ctx context.Context) (*app, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	_, tracerShutdown, err := telemetry.New(ctx, cfg.Telemetry.OTLPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("init telemetry: %w", err)
	}

	pool, err := buildPool(ctx, cfg.Postgres)
	if err != nil {
		_ = tracerShutdown(ctx)
		return nil, err
	}

	engine, err := engineclient.New(cfg.Engine.Addr)
	if err != nil {
		pool.Close()
		_ = tracerShutdown(ctx)
		return nil, err
	}

	repo := repopostgres.NewRepository(pool)
	wallet := walletclient.New(cfg.Wallet.BaseURL, cfg.Wallet.InternalToken)
	service := application.NewService(repo, engine.GameEngine(), wallet)
	hub := deliverygrpc.NewHub()
	validator := deliverygrpc.NewJWTValidator(cfg.Auth.Issuer, cfg.Auth.JWKSURL)
	authenticator := deliverygrpc.NewAuthenticator(validator, cfg.Auth.AllowLegacyUserHeader)
	server := deliverygrpc.NewServer(service, hub, authenticator)

	return &app{
		cfg:            cfg,
		grpcServer:     buildGRPCServer(server),
		httpServer:     buildHTTPServer(ctx, cfg.GRPC.Addr, cfg.HTTP.Addr, server, hub, validator, cfg.Auth.AllowLegacyUserHeader),
		metricsServer:  buildMetricsServer(cfg.Metrics.Addr),
		pool:           pool,
		engine:         engine,
		tracerShutdown: tracerShutdown,
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
	a.grpcServer.GracefulStop()
	if err := a.httpServer.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown failed", zap.Error(err))
	}
	if err := a.metricsServer.Shutdown(ctx); err != nil {
		logger.Error("metrics server shutdown failed", zap.Error(err))
	}
	if err := a.tracerShutdown(ctx); err != nil {
		logger.Error("tracer shutdown failed", zap.Error(err))
	}
	a.pool.Close()
	if err := a.engine.Close(); err != nil {
		logger.Error("engine client shutdown failed", zap.Error(err))
	}
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
