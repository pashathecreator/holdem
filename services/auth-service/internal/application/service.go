package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/pashathecreator/holdem/services/auth-service/internal/domain"
	"github.com/pashathecreator/holdem/services/auth-service/internal/security"
)

type Repository interface {
	CreateUser(ctx context.Context, user *domain.User, passwordHash string) error
	FindUserWithPasswordByEmail(ctx context.Context, email string) (*domain.UserWithPassword, error)
	FindUserByID(ctx context.Context, userID string) (*domain.User, error)
	SearchUsersByEmail(ctx context.Context, query string, limit int) ([]*domain.User, error)
	CreateRefreshSession(ctx context.Context, session *domain.RefreshSession) error
	FindRefreshSessionByTokenHash(ctx context.Context, tokenHash string) (*domain.RefreshSession, error)
	RotateRefreshSession(ctx context.Context, oldSessionID string, newSession *domain.RefreshSession) error
	RevokeRefreshSession(ctx context.Context, tokenHash string) error
}

type TokenManager interface {
	IssueAccessToken(userID string, isAdmin bool) (string, error)
	NewRefreshToken() (string, string, time.Time, error)
	HashRefreshToken(token string) string
}

type UserCreatedPublisher interface {
	PublishUserCreated(ctx context.Context, user *domain.User) error
}

type AuthResult struct {
	AccessToken  string
	RefreshToken string
	User         *domain.User
}

type Service struct {
	repo      Repository
	tokens    TokenManager
	publisher UserCreatedPublisher
	admins    map[string]struct{}
}

func NewService(repo Repository, tokens TokenManager, publisher UserCreatedPublisher, adminEmails []string) *Service {
	admins := make(map[string]struct{}, len(adminEmails))
	for _, email := range adminEmails {
		admins[normalizeEmail(email)] = struct{}{}
	}
	return &Service{repo: repo, tokens: tokens, publisher: publisher, admins: admins}
}

func (s *Service) Register(ctx context.Context, email, password string) (*AuthResult, error) {
	email = normalizeEmail(email)
	if !isValidEmail(email) || len(password) < 8 {
		return nil, domain.ErrInvalidInput
	}

	passwordHash, err := security.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		ID:        newID("user"),
		Email:     email,
		IsAdmin:   s.isAdminEmail(email),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateUser(ctx, user, passwordHash); err != nil {
		return nil, err
	}
	if s.publisher != nil {
		_ = s.publisher.PublishUserCreated(ctx, user)
	}

	return s.issueSession(ctx, user)
}

func (s *Service) Login(ctx context.Context, email, password string) (*AuthResult, error) {
	record, err := s.repo.FindUserWithPasswordByEmail(ctx, normalizeEmail(email))
	if err != nil {
		return nil, err
	}
	if !security.CheckPassword(record.PasswordHash, password) {
		return nil, domain.ErrInvalidCredentials
	}

	return s.issueSession(ctx, record.User)
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (*AuthResult, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, domain.ErrInvalidRefreshToken
	}

	session, err := s.repo.FindRefreshSessionByTokenHash(ctx, s.tokens.HashRefreshToken(refreshToken))
	if err != nil {
		return nil, err
	}
	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now().UTC()) {
		return nil, domain.ErrInvalidRefreshToken
	}

	user, err := s.repo.FindUserByID(ctx, session.UserID)
	if err != nil {
		return nil, err
	}

	accessToken, err := s.tokens.IssueAccessToken(user.ID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	newRefreshToken, refreshHash, refreshExpiresAt, err := s.tokens.NewRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}
	newSession := &domain.RefreshSession{
		ID:        newID("refresh"),
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: refreshExpiresAt,
	}
	if err := s.repo.RotateRefreshSession(ctx, session.ID, newSession); err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		User:         user,
	}, nil
}

func (s *Service) Me(ctx context.Context, userID string) (*domain.User, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrUserNotFound
	}
	return s.repo.FindUserByID(ctx, userID)
}

func (s *Service) SearchUsersByEmail(ctx context.Context, query string, limit int) ([]*domain.User, error) {
	query = normalizeEmail(query)
	if query == "" {
		return nil, domain.ErrInvalidInput
	}
	if limit <= 0 {
		limit = 20
	}
	return s.repo.SearchUsersByEmail(ctx, query, limit)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if strings.TrimSpace(refreshToken) == "" {
		return domain.ErrInvalidRefreshToken
	}
	return s.repo.RevokeRefreshSession(ctx, s.tokens.HashRefreshToken(refreshToken))
}

func (s *Service) issueSession(ctx context.Context, user *domain.User) (*AuthResult, error) {
	accessToken, err := s.tokens.IssueAccessToken(user.ID, user.IsAdmin)
	if err != nil {
		return nil, fmt.Errorf("issue access token: %w", err)
	}
	refreshToken, refreshHash, refreshExpiresAt, err := s.tokens.NewRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("issue refresh token: %w", err)
	}
	if err := s.repo.CreateRefreshSession(ctx, &domain.RefreshSession{
		ID:        newID("refresh"),
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		return nil, err
	}

	return &AuthResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         user,
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isValidEmail(email string) bool {
	return strings.Contains(email, "@") && !strings.HasPrefix(email, "@") && !strings.HasSuffix(email, "@")
}

func newID(prefix string) string {
	raw := make([]byte, 12)
	_, err := rand.Read(raw)
	if err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + strings.ToLower(base64.RawURLEncoding.EncodeToString(raw))
}

func (s *Service) isAdminEmail(email string) bool {
	_, ok := s.admins[normalizeEmail(email)]
	return ok
}
