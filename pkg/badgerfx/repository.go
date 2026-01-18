package badgerfx

import (
	"fmt"

	"github.com/dgraph-io/badger/v4"
)

type EntityFactory[T Entity] func() T

type Repository[T Entity] struct {
	zero    T
	factory EntityFactory[T]
}

func NewRepository[T Entity](factory EntityFactory[T]) *Repository[T] {
	var zero T
	return &Repository[T]{
		zero:    zero,
		factory: factory,
	}
}

func (r *Repository[T]) List(txn *badger.Txn, prefix string, options badger.IteratorOptions) ([]T, error) {
	validPrefix := []byte(prefix)
	seekPrefix := []byte(prefix)
	if options.Reverse {
		seekPrefix = append(seekPrefix, SeekEnd)
	}

	it := txn.NewIterator(options)
	defer it.Close()

	var entities []T
	for it.Seek(seekPrefix); it.ValidForPrefix(validPrefix); it.Next() {
		item := it.Item()

		entity := r.factory()
		if err := item.Value(func(val []byte) error {
			return entity.UnmarshalStorage(val)
		}); err != nil {
			return nil, fmt.Errorf("failed to unmarshal entity: %w", err)
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

func (r *Repository[T]) Read(txn *badger.Txn, id string) (T, error) {
	key := []byte(r.zero.StorageKey(id))
	item, err := txn.Get(key)
	if err != nil {
		return r.zero, fmt.Errorf("failed to get entity: %w", err)
	}

	entity := r.factory()
	if valErr := item.Value(func(val []byte) error {
		return entity.UnmarshalStorage(val)
	}); valErr != nil {
		return r.zero, fmt.Errorf("failed to unmarshal entity: %w", valErr)
	}

	return entity, nil
}

func (r *Repository[T]) ReadByIndex(txn *badger.Txn, index string) (T, error) {
	item, err := txn.Get([]byte(index))
	if err != nil {
		return r.zero, fmt.Errorf("failed to get entity: %w", err)
	}

	key, err := item.ValueCopy(nil)
	if err != nil {
		return r.zero, fmt.Errorf("failed to get entity key: %w", err)
	}

	item, err = txn.Get(key)
	if err != nil {
		return r.zero, fmt.Errorf("failed to get entity: %w", err)
	}

	entity := r.factory()
	if valErr := item.Value(func(val []byte) error {
		return entity.UnmarshalStorage(val)
	}); valErr != nil {
		return r.zero, fmt.Errorf("failed to unmarshal entity: %w", valErr)
	}

	return entity, nil
}

func (r *Repository[T]) Write(txn *badger.Txn, entity T) error {
	data, err := entity.MarshalStorage()
	if err != nil {
		return fmt.Errorf("failed to marshal entity: %w", err)
	}

	if indexErr := r.CreateIndexes(txn, entity); indexErr != nil {
		return indexErr
	}

	if setErr := txn.Set([]byte(entity.StorageKey()), data); setErr != nil {
		return fmt.Errorf("failed to update entity: %w", setErr)
	}

	return nil
}

func (r *Repository[T]) Delete(txn *badger.Txn, id string) error {
	entity, err := r.Read(txn, id)
	if err != nil {
		return err
	}

	if indexErr := r.DeleteIndexes(txn, entity); indexErr != nil {
		return indexErr
	}

	if delErr := txn.Delete([]byte(entity.StorageKey())); delErr != nil {
		return fmt.Errorf("failed to delete entity: %w", delErr)
	}

	return nil
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
