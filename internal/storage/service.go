package storage

import (
	"context"
	"path/filepath"

	"go.uber.org/zap"
)

// Storage provides an interface for data persistence operations.
type Storage interface {
	Initialize(ctx context.Context) error
	Close(ctx context.Context) error
}

// storage is the internal implementation of the Storage interface.
type storage struct {
	config Config

	logger *zap.Logger
}

// New creates a new storage instance.
func New(cfg Config, logger *zap.Logger) Storage {
	return &storage{
		logger: logger,
		config: cfg,
	}
}

// Initialize initializes the storage backend.
func (s *storage) Initialize(_ context.Context) error {
	s.logger.Info("initializing storage backend",
		zap.String("data_dir", s.config.DataDir),
	)

	// TODO: Initialize BadgerDB or other storage backend here
	// For now, just create the data directory if it doesn't exist
	// and log the initialization

	return nil
}

// Close gracefully closes the storage backend.
func (s *storage) Close(_ context.Context) error {
	s.logger.Info("closing storage backend")

	// TODO: Close BadgerDB or other storage backend here
	// For now, just log the shutdown

	return nil
}

// GetDataDir returns the configured data directory for storage.
func (s *storage) GetDataDir() string {
	return s.config.DataDir
}

// EnsureDir ensures that the specified directory exists.
func (s *storage) EnsureDir(dir string) error {
	// TODO: Implement directory creation logic
	s.logger.Debug("ensuring directory exists", zap.String("dir", dir))
	return nil
}

// GetStoragePath returns the full path for a storage file.
func (s *storage) GetStoragePath(filename string) string {
	return filepath.Join(s.config.DataDir, filename)
}
