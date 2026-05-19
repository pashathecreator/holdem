package main

import (
	"net/http"

	deliveryhttp "github.com/pashathecreator/holdem/services/auth-service/internal/delivery/http"
)

func buildHTTPServer(addr string, server *deliveryhttp.Server) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", server.Healthz)
	mux.HandleFunc("POST /v1/auth/register", server.Register)
	mux.HandleFunc("POST /v1/auth/login", server.Login)
	mux.HandleFunc("POST /v1/auth/refresh", server.Refresh)
	mux.HandleFunc("POST /v1/auth/logout", server.Logout)
	mux.HandleFunc("GET /v1/auth/me", server.Me)
	mux.HandleFunc("GET /v1/admin/users/search", server.AdminUsersSearch)
	mux.HandleFunc("GET /.well-known/jwks.json", server.JWKS)

	return &http.Server{
		Addr:    addr,
		Handler: mux,
	}
}
