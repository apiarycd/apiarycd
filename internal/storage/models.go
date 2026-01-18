package storage

import (
	"time"

	"github.com/google/uuid"
)

type Entity interface {
	StorageKey(id ...string) string
	StorageIndexes() []string

	MarshalStorage() ([]byte, error)
	UnmarshalStorage([]byte) error
}

// BaseEntity provides common fields for all storage entities.
type BaseEntity struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
