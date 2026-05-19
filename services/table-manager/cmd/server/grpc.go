package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	grpcprom "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	grpc_health_v1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	deliverygrpc "github.com/pashathecreator/holdem/services/table-manager/internal/delivery/grpc"
	tablemanagerv1 "github.com/pashathecreator/holdem/services/table-manager/pkg/gen/go/table_manager/v1"
)

func buildGRPCServer(server *deliverygrpc.Server) *grpc.Server {
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(grpcprom.UnaryServerInterceptor),
		grpc.ChainStreamInterceptor(grpcprom.StreamServerInterceptor),
	)
	tablemanagerv1.RegisterTableManagerServer(grpcServer, server)
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	grpcprom.Register(grpcServer)
	reflection.Register(grpcServer)
	return grpcServer
}

func buildHTTPServer(ctx context.Context, grpcAddr, httpAddr string, server *deliverygrpc.Server, hub *deliverygrpc.Hub, validator *deliverygrpc.JWTValidator, allowLegacy bool) *http.Server {
	gw := runtime.NewServeMux(
		runtime.WithIncomingHeaderMatcher(func(key string) (string, bool) {
			if key == "Authorization" || key == "authorization" {
				return "authorization", true
			}
			if key == "X-User-Id" || key == "x-user-id" {
				return "x-user-id", true
			}
			return runtime.DefaultHeaderMatcher(key)
		}),
	)
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if err := tablemanagerv1.RegisterTableManagerHandlerFromEndpoint(ctx, gw, grpcAddr, opts); err != nil {
		panic(fmt.Sprintf("failed to register gateway: %v", err))
	}

	mux := http.NewServeMux()
	mux.Handle("/v1/", authHTTPMiddleware(validator, allowLegacy, gw))
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", http.FileServer(http.Dir("docs"))))
	mux.Handle("/v1/tables/", authHTTPMiddleware(validator, allowLegacy, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && len(r.URL.Path) > len("/v1/tables/") && r.URL.Path[len(r.URL.Path)-3:] == "/ws" {
			tableID := r.URL.Path[len("/v1/tables/") : len(r.URL.Path)-3]
			r.SetPathValue("table_id", tableID)
			deliverygrpc.NewWSHandler(hub, deliverygrpc.NewWSSnapshotAdapter(server)).ServeHTTP(w, r)
			return
		}
		gw.ServeHTTP(w, r)
	})))

	return &http.Server{
		Addr:    httpAddr,
		Handler: otelhttp.NewHandler(mux, "table-manager-http"),
	}
}

func authHTTPMiddleware(validator *deliverygrpc.JWTValidator, allowLegacy bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		required := routeRequiresAuth(r)
		userID, err := validator.AuthenticateHTTPRequest(r, required, allowLegacy)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if userID != "" {
			r = deliverygrpc.InjectUserID(r, userID)
		}
		next.ServeHTTP(w, r)
	})
}

func routeRequiresAuth(r *http.Request) bool {
	if r.Method == http.MethodPost {
		if r.URL.Path == "/v1/tables" {
			return true
		}
		if strings.HasPrefix(r.URL.Path, "/v1/tables/") {
			return true
		}
	}
	return r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/ws")
}

func buildListener(addr string) (net.Listener, error) {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen %s: %w", addr, err)
	}
	return lis, nil
}
