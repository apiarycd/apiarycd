package auth

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// UserRole represents the role of a user in the system
type UserRole string

const (
	UserRoleAdmin   UserRole = "admin"
	UserRoleUser    UserRole = "user"
	UserRoleService UserRole = "service"
)

// JWTClaims represents the claims stored in a JWT token
type JWTClaims struct {
	jwt.RegisteredClaims
	UserID string   `json:"user_id"`
	Role   UserRole `json:"role"`
}

// NewJWTClaims creates a new JWTClaims instance
func NewJWTClaims(userID string, role UserRole, expiresAt time.Time) *JWTClaims {
	return &JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        uuid.NewString(),
		},
		UserID: userID,
		Role:   role,
	}
}
