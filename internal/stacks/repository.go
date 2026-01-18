package stacks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apiarycd/apiarycd/pkg/badgerfx"
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

type Repository struct {
	storage *badgerfx.Repository[*stackModel]

	db *badger.DB
}

func NewRepository(db *badger.DB) *Repository {
	return &Repository{
		storage: badgerfx.NewRepository(func() *stackModel { return new(stackModel) }),

		db: db,
	}
}

// Create creates a new stack.
func (r *Repository) Create(_ context.Context, stack *StackDraft) (*Stack, error) {
	model := newStackModel(stack)
	err := r.db.Update(func(txn *badger.Txn) error {
		_, err := r.storage.ReadByIndex(txn, model.nameIndex())
		if err == nil {
			return fmt.Errorf("%w: stack with name %q already exists", ErrConflict, model.Name)
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to check name uniqueness: %w", err)
		}

		if err := r.storage.Write(txn, model); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create stack: %w", err)
	}

	return model.toDomain(), nil
}

// GetByID retrieves a stack by its ID.
func (r *Repository) GetByID(_ context.Context, id uuid.UUID) (*Stack, error) {
	var model *stackModel

	err := r.db.View(func(txn *badger.Txn) error {
		var err error
		model, err = r.storage.Read(txn, id.String())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get stack by ID: %w", err)
	}

	return model.toDomain(), nil
}

// Update updates an existing stack.
func (r *Repository) Update(_ context.Context, id uuid.UUID, updater func(*Stack) error) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		old, err := r.storage.Read(txn, id.String())
		if err != nil {
			return fmt.Errorf("failed to get stack before update: %w", err)
		}

		stack := old.toDomain()

		if updErr := updater(stack); updErr != nil {
			return fmt.Errorf("failed to update stack: %w", updErr)
		}

		model := newStackModel(&stack.StackDraft)
		model.ID = old.ID
		model.CreatedAt = old.CreatedAt
		model.UpdatedAt = time.Now()

		if indexErr := r.storage.DeleteIndexes(txn, old); indexErr != nil {
			return indexErr
		}

		if writeErr := r.storage.Write(txn, model); writeErr != nil {
			return writeErr
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	return nil
}

// Delete deletes a stack.
func (r *Repository) Delete(_ context.Context, id uuid.UUID) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		return r.storage.Delete(txn, id.String())
	})

	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}

// List retrieves all stacks.
func (r *Repository) List(_ context.Context) ([]Stack, error) {
	var stacks []Stack

	err := r.db.View(func(txn *badger.Txn) error {
		items, err := r.storage.List(txn, prefixByID, badger.DefaultIteratorOptions)
		if err != nil {
			return err
		}

		for _, item := range items {
			stacks = append(stacks, *item.toDomain())
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list stacks: %w", err)
	}

	return stacks, nil
}
