package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// AuthService implements AuthService
type AuthService struct {
	config Config

	userRepository   *badgerUserRepository
	apiKeyRepository *badgerAPIKeyRepository
}

// NewAuthService creates a new AuthService
func NewAuthService(
	config Config,
	userRepository *badgerUserRepository,
	apiKeyRepository *badgerAPIKeyRepository,
) *AuthService {
	return &AuthService{
		config: config,

		userRepository:   userRepository,
		apiKeyRepository: apiKeyRepository,
	}
}

// AuthenticateUser authenticates a user by email and password
func (s *AuthService) AuthenticateUser(ctx context.Context, name, password string) (*User, string, error) {
	// Get user by email
	user, err := s.userRepository.GetByName(ctx, name)
	if err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", ErrInvalidCredentials
	}

	// Generate JWT token
	token, err := s.GenerateJWT(ctx, user)
	if err != nil {
		return nil, "", err
	}

	return user, token, nil
}

// GenerateJWT generates a JWT token for a user
func (s *AuthService) GenerateJWT(ctx context.Context, user *User) (string, error) {
	// Create claims
	claims := NewJWTClaims(user.ID.String(), user.Role, time.Now().Add(s.config.AccessTokenExp))

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token
	return token.SignedString(s.config.SecretKey)
}

// ValidateJWT validates a JWT token and returns the claims
func (s *AuthService) ValidateJWT(ctx context.Context, tokenString string) (*JWTClaims, error) {
	// Parse token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.config.SecretKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}), jwt.WithExpirationRequired(), jwt.WithIssuer(s.config.Issuer), jwt.WithIssuedAt())

	if err != nil {
		if errors.Is(err, jwt.ErrSignatureInvalid) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}

	// Check if token is valid
	if !token.Valid {
		return nil, ErrTokenInvalid
	}

	// Extract claims
	claims, ok := token.Claims.(*JWTClaims)
	if !ok {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// RefreshJWT refreshes a JWT token
func (s *AuthService) RefreshJWT(ctx context.Context, token string) (string, error) {
	// Validate the existing token
	claims, err := s.ValidateJWT(ctx, token)
	if err != nil {
		return "", err
	}

	// Create new claims with extended expiration
	newClaims := NewJWTClaims(
		claims.UserID,
		claims.Role,
		time.Now().Add(s.config.AccessTokenExp),
	)

	// Create new token
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, newClaims)

	// Sign new token
	return newToken.SignedString(s.config.SecretKey)
}
