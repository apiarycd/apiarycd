package repositories

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrEmptyStorageDir = errors.New("storage dir is empty")
	ErrInvalidTimeout  = errors.New("timeout must be greater than 0")
)

type Config struct {
	StorageDir string
	Timeout    time.Duration
	Auth       GitAuth
}

func (c Config) Validate() error {
	if c.StorageDir == "" {
		return ErrEmptyStorageDir
	}

	if c.Timeout <= 0 {
		return ErrInvalidTimeout
	}

	if err := c.Auth.SSH.Validate(); err != nil {
		return fmt.Errorf("invalid SSH auth config: %w", err)
	}

	return nil
}
