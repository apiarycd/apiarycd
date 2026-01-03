package badgerfx

import "github.com/dgraph-io/badger/v4"

type Config struct {
	// Path to the BadgerDB data directory
	Dir string
}

func (c Config) Build() badger.Options {
	options := badger.DefaultOptions(c.Dir)

	return options
}
