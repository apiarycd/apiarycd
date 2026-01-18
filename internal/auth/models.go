package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/apiarycd/apiarycd/internal/storage"
	"github.com/google/uuid"
)

// userModel represents a user in the system
type userModel struct {
	storage.BaseEntity

	Name         string   `json:"name"`
	PasswordHash string   `json:"password_hash"`
	Role         UserRole `json:"role"`
}

func newUserModel(user UserDraft, passwordHash string) *userModel {
	return &userModel{
		BaseEntity: storage.BaseEntity{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:         user.Name,
		PasswordHash: passwordHash,
		Role:         user.Role,
	}
}

// apiKeyModel represents an API key for service authentication
type apiKeyModel struct {
	storage.BaseEntity

	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	RevokedAt time.Time `json:"revoked_at"`
}

func newAPIKeyModel(apiKey APIKeyDraft) *apiKeyModel {
	return &apiKeyModel{
		BaseEntity: storage.BaseEntity{
			ID:        apiKey.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		UserID:    apiKey.UserID,
		ExpiresAt: apiKey.ExpiresAt,
		RevokedAt: time.Time{},
	}
}

// ToBadgerKey converts a User to a BadgerDB key
func (u *userModel) ToBadgerKey() []byte {
	return []byte("user:" + u.ID.String())
}

func (u *userModel) indexes() []string {
	return []string{"user:name:" + u.Name}
}

// ToBadgerValue converts a User to a BadgerDB value
func (u *userModel) ToBadgerValue() ([]byte, error) {
	data, err := json.Marshal(u)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user: %w", err)
	}

	return data, nil
}

// FromBadgerValue parses a User from a BadgerDB value
func (u *userModel) FromBadgerValue(value []byte) error {
	if err := json.Unmarshal(value, u); err != nil {
		return fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return nil
}

func (u *userModel) toDomain() *User {
	if u == nil {
		return nil
	}

	return &User{
		UserBase: UserBase{
			Name:         u.Name,
			Role:         u.Role,
			PasswordHash: u.PasswordHash,
		},
		ID:        u.ID,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

func (u *userModel) update(user UserBase) {
	u.Role = user.Role
	u.UpdatedAt = time.Now()
}

// ToBadgerKey converts an APIKey to a BadgerDB key
func (ak *apiKeyModel) ToBadgerKey() []byte {
	return []byte("apikey:" + ak.ID.String())
}

func (ak *apiKeyModel) indexes() []string {
	return []string{"apikey:user:" + ak.UserID.String() + ":" + ak.ID.String()}
}

// ToBadgerValue converts an APIKey to a BadgerDB value
func (ak *apiKeyModel) ToBadgerValue() ([]byte, error) {
	data, err := json.Marshal(ak)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal API key: %w", err)
	}

	return data, nil
}

// FromBadgerValue parses an APIKey from a BadgerDB value
func (ak *apiKeyModel) FromBadgerValue(value []byte) error {
	if err := json.Unmarshal(value, ak); err != nil {
		return fmt.Errorf("failed to unmarshal API key: %w", err)
	}

	return nil
}

func (ak *apiKeyModel) toDomain() *APIKey {
	if ak == nil {
		return nil
	}

	return &APIKey{
		APIKeyDraft: APIKeyDraft{
			ID:        ak.ID,
			UserID:    ak.UserID,
			CreatedAt: ak.CreatedAt,
			ExpiresAt: ak.ExpiresAt,
		},
		UpdatedAt: ak.UpdatedAt,
		RevokedAt: ak.RevokedAt,
	}
}

func (ak *apiKeyModel) update(apiKey APIKeyDraft) {
	ak.ExpiresAt = apiKey.ExpiresAt
	ak.UpdatedAt = time.Now()
}
