package deliveryhttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/pashathecreator/holdem/services/auth-service/internal/application"
	"github.com/pashathecreator/holdem/services/auth-service/internal/domain"
	"github.com/pashathecreator/holdem/services/auth-service/internal/security"
)

type authService interface {
	Register(rctx context.Context, email, password string) (*application.AuthResult, error)
	Login(rctx context.Context, email, password string) (*application.AuthResult, error)
	Refresh(rctx context.Context, refreshToken string) (*application.AuthResult, error)
	Logout(rctx context.Context, refreshToken string) error
	Me(rctx context.Context, userID string) (*domain.User, error)
	SearchUsersByEmail(rctx context.Context, query string, limit int) ([]*domain.User, error)
}

type tokenParser interface {
	ParseAccessToken(token string) (*security.Claims, error)
	JWKS() security.JWKResponse
}

type Server struct {
	service authService
	tokens  tokenParser
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type userSearchResponse struct {
	Users []*userResponse `json:"users"`
}

type authResponse struct {
	AccessToken  string        `json:"access_token"`
	RefreshToken string        `json:"refresh_token"`
	User         *userResponse `json:"user"`
}

type userResponse struct {
	UserID    string `json:"user_id"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

func NewServer(service authService, tokens tokenParser) *Server {
	return &Server{service: service, tokens: tokens}
}

func (s *Server) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.service.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toAuthResponse(result))
}

func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(result))
}

func (s *Server) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toAuthResponse(result))
}

func (s *Server) Logout(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.service.Logout(r.Context(), req.RefreshToken); err != nil {
		writeDomainError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) Me(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, errors.New("missing bearer token"))
		return
	}

	claims, err := s.tokens.ParseAccessToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return
	}

	user, err := s.service.Me(r.Context(), claims.Sub)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toUserResponse(user))
}

func (s *Server) JWKS(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.tokens.JWKS())
}

func (s *Server) AdminUsersSearch(w http.ResponseWriter, r *http.Request) {
	claims, ok := s.adminClaims(w, r)
	if !ok {
		return
	}
	if !claims.IsAdmin {
		writeError(w, http.StatusForbidden, errors.New("admin access required"))
		return
	}

	users, err := s.service.SearchUsersByEmail(r.Context(), r.URL.Query().Get("email"), 20)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	result := make([]*userResponse, 0, len(users))
	for _, user := range users {
		result = append(result, toUserResponse(user))
	}
	writeJSON(w, http.StatusOK, &userSearchResponse{Users: result})
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err)
	case errors.Is(err, domain.ErrEmailTaken):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, domain.ErrInvalidCredentials), errors.Is(err, domain.ErrInvalidRefreshToken):
		writeError(w, http.StatusUnauthorized, err)
	case errors.Is(err, domain.ErrUserNotFound):
		writeError(w, http.StatusNotFound, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}

func writeError(w http.ResponseWriter, statusCode int, err error) {
	writeJSON(w, statusCode, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, statusCode int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(r *http.Request, value interface{}) error {
	return json.NewDecoder(r.Body).Decode(value)
}

func toAuthResponse(result *application.AuthResult) *authResponse {
	return &authResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		User:         toUserResponse(result.User),
	}
}

func toUserResponse(user *domain.User) *userResponse {
	return &userResponse{
		UserID:    user.ID,
		Email:     user.Email,
		IsAdmin:   user.IsAdmin,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) adminClaims(w http.ResponseWriter, r *http.Request) (*security.Claims, bool) {
	token := bearerToken(r.Header.Get("Authorization"))
	if token == "" {
		writeError(w, http.StatusUnauthorized, errors.New("missing bearer token"))
		return nil, false
	}

	claims, err := s.tokens.ParseAccessToken(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err)
		return nil, false
	}
	return claims, true
}

func bearerToken(header string) string {
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
