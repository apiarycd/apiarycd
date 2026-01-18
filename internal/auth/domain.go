package auth

import (
	"time"

	"github.com/google/uuid"
)

type UserBase struct {
	Name         string
	Role         UserRole
	PasswordHash string
}

type UserDraft struct {
	UserBase

	Password string
}

type User struct {
	UserBase

	ID        uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
}

type APIKeyDraft struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

type APIKey struct {
	APIKeyDraft

	UpdatedAt time.Time
	RevokedAt time.Time
}
