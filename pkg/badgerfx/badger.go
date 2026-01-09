package badgerfx

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

const SeekEnd = byte(0xFF)

func New(config Config, logger *zapLogger) (*badger.DB, error) {
	opts := config.Build().
		WithLogger(logger)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	return db, nil
}
