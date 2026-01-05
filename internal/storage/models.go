package storage

import (
	"time"

	"github.com/google/uuid"
)

// BaseEntity provides common fields for all storage entities.
type BaseEntity struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
