package domain

import (
	"errors"
	"time"
)

type User struct {
	ID        string
	Email     string
	IsAdmin   bool
	CreatedAt time.Time
}

type UserWithPassword struct {
	User         *User
	PasswordHash string
}

type RefreshSession struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
}

var (
	ErrEmailTaken          = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrInvalidInput        = errors.New("invalid input")
	ErrUserNotFound        = errors.New("user not found")
)
