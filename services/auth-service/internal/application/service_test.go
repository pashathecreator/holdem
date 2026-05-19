package application

import (
	"context"
	"testing"
	"time"

	"github.com/pashathecreator/holdem/services/auth-service/internal/domain"
)

type authRepoStub struct {
	createdUser *domain.User
}

func (s *authRepoStub) CreateUser(_ context.Context, user *domain.User, _ string) error {
	s.createdUser = user
	return nil
}
func (s *authRepoStub) FindUserWithPasswordByEmail(context.Context, string) (*domain.UserWithPassword, error) {
	return nil, domain.ErrInvalidCredentials
}
func (s *authRepoStub) FindUserByID(context.Context, string) (*domain.User, error) {
	return nil, domain.ErrUserNotFound
}
func (s *authRepoStub) SearchUsersByEmail(context.Context, string, int) ([]*domain.User, error) {
	return nil, nil
}
func (s *authRepoStub) CreateRefreshSession(_ context.Context, _ *domain.RefreshSession) error {
	return nil
}
func (s *authRepoStub) FindRefreshSessionByTokenHash(context.Context, string) (*domain.RefreshSession, error) {
	return nil, domain.ErrInvalidRefreshToken
}
func (s *authRepoStub) RotateRefreshSession(context.Context, string, *domain.RefreshSession) error {
	return nil
}
func (s *authRepoStub) RevokeRefreshSession(context.Context, string) error { return nil }

type tokenManagerStub struct {
	isAdmin bool
}

func (s *tokenManagerStub) IssueAccessToken(_ string, isAdmin bool) (string, error) {
	s.isAdmin = isAdmin
	return "access", nil
}
func (s *tokenManagerStub) NewRefreshToken() (string, string, time.Time, error) {
	return "refresh", "hash", time.Now().Add(time.Hour), nil
}
func (s *tokenManagerStub) HashRefreshToken(token string) string { return token }

func TestRegisterMarksConfiguredAdminEmail(t *testing.T) {
	repo := &authRepoStub{}
	tokens := &tokenManagerStub{}
	service := NewService(repo, tokens, nil, []string{"admin@example.com"})

	result, err := service.Register(context.Background(), "admin@example.com", "password123")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.User == nil || !result.User.IsAdmin {
		t.Fatalf("result.User.IsAdmin = %v, want true", result.User != nil && result.User.IsAdmin)
	}
	if repo.createdUser == nil || !repo.createdUser.IsAdmin {
		t.Fatalf("created user admin flag was not persisted")
	}
	if !tokens.isAdmin {
		t.Fatalf("IssueAccessToken() should be called with admin=true")
	}
}
