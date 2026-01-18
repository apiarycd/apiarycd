package storage

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type EntityFactory[T Entity] func() T

type Repository[T Entity] struct {
	db      *badger.DB
	factory EntityFactory[T]
}

func NewRepository[T Entity](db *badger.DB, factory EntityFactory[T]) *Repository[T] {
	return &Repository[T]{
		db:      db,
		factory: factory,
	}
}

func (r *Repository[T]) Read(txn *badger.Txn, id string) (*T, error) {
	key := []byte(id)
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	var entity T
	if err := item.Value(func(val []byte) error {
		return entity.UnmarshalStorage(val)
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	return &entity, nil
}

func (r *Repository[T]) ReadByIndex(txn *badger.Txn, index string) (*T, error) {
	item, err := txn.Get([]byte(index))
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	key, err := item.ValueCopy(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity key: %w", err)
	}

	item, err = txn.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get entity: %w", err)
	}

	var entity T
	if err := item.Value(func(val []byte) error {
		return entity.UnmarshalStorage(val)
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
	}

	return &entity, nil
}

func (r *Repository[T]) Write(txn *badger.Txn, entity T) error {
	data, err := entity.MarshalStorage()
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	if err := r.CreateIndexes(txn, entity); err != nil {
		return err
	}

	return txn.Set([]byte(entity.StorageKey()), data)
}

func (r *Repository[T]) CreateIndexes(txn *badger.Txn, entity T) error {
	key := []byte(entity.StorageKey())
	for _, index := range entity.StorageIndexes() {
		if err := txn.Set([]byte(index), key); err != nil {
			return fmt.Errorf("failed to set entity index: %w", err)
		}
	}

	return nil
}

func (r *Repository[T]) DeleteIndexes(txn *badger.Txn, entity T) error {
	for _, index := range entity.StorageIndexes() {
		if err := txn.Delete([]byte(index)); err != nil {
			return fmt.Errorf("failed to delete entity index: %w", err)
		}
	}

	return nil
}
