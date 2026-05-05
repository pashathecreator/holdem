package main

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"

	"github.com/pashathecreator/holdem/services/engine/internal/application"
	deliverygrpc "github.com/pashathecreator/holdem/services/engine/internal/delivery/grpc"
	"github.com/pashathecreator/holdem/services/engine/internal/domain"
	enginev1 "github.com/pashathecreator/holdem/services/engine/pkg/gen/go/engine/v1"
)

type gameStateReader interface {
	FindByID(ctx context.Context, id domain.HandID) (*domain.GameState, error)
}

func buildGRPCServer(
	startHand *application.StartHand,
	applyAction *application.ApplyAction,
	finishHand *application.FinishHand,
	repo gameStateReader,
	pubsub *deliverygrpc.PubSub,
) *grpc.Server {
	server := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)

	enginev1.RegisterGameEngineServer(
		server,
		deliverygrpc.NewServer(startHand, applyAction, finishHand, repo, pubsub),
	)

	grpcprom.Register(server)
	reflection.Register(server)

	return server
}

func buildHTTPServer(ctx context.Context, grpcAddr, httpAddr string) *http.Server {
	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	if err := enginev1.RegisterGameEngineHandlerFromEndpoint(ctx, mux, grpcAddr, opts); err != nil {
		panic(fmt.Sprintf("failed to register gateway: %v", err))
	}

	return &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}
}

func buildListener(addr string) (net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return lis, nil
}