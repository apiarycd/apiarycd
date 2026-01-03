package stacks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	prefix = "stack:"

	prefixByID     = prefix + "id:"
	prefixByName   = prefix + "name:"
	prefixByStatus = prefix + "status:"
	prefixByLabel  = prefix + "label:"
)

type Repository struct {
	db     *badger.DB
	logger *zap.Logger
}

func NewRepository(db *badger.DB, logger *zap.Logger) *Repository {
	return &Repository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new stack.
func (r *Repository) Create(_ context.Context, stack *StackDraft) error {
	model := newStackModel(stack)

	// Serialize the stack
	data, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal stack: %w", err)
	}

	err = r.db.Update(func(txn *badger.Txn) error {
		// Store the stack
		key := r.getStackKey(model.ID)
		if setErr := txn.Set(key, data); setErr != nil {
			return fmt.Errorf("failed to store stack: %w", setErr)
		}

		// Create indexes
		if crErr := r.createStackIndexes(txn, model); crErr != nil {
			return fmt.Errorf("failed to create stack indexes: %w", crErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create stack: %w", err)
	}

	return nil
}

// GetByID retrieves a stack by its ID.
func (r *Repository) GetByID(_ context.Context, id uuid.UUID) (*Stack, error) {
	var stack *stackModel

	err := r.db.View(func(txn *badger.Txn) error {
		found, err := r.getByID(txn, id)
		if err == nil {
			stack = found
		}

		return err
	})

	return newStack(stack), err
}

// GetByName retrieves a stack by its name.
func (r *Repository) GetByName(ctx context.Context, name string) (*Stack, error) {
	var stackID uuid.UUID

	err := r.db.View(func(txn *badger.Txn) error {
		key := r.getStackNameKey(name)
		item, err := txn.Get(key)
		if errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("%w: %s", ErrNotFound, name)
		}
		if err != nil {
			return fmt.Errorf("failed to get stack name index: %w", err)
		}

		// Get the actual stack ID
		if valErr := item.Value(func(val []byte) error { return json.Unmarshal(val, &stackID) }); valErr != nil {
			return fmt.Errorf("failed to get stack ID: %w", valErr)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get stack by name: %w", err)
	}

	return r.GetByID(ctx, stackID)
}

// Update updates an existing stack.
func (r *Repository) Update(_ context.Context, id uuid.UUID, updater func(*Stack) error) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		old, err := r.getByID(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get stack before update: %w", err)
		}

		stack := newStack(old)

		if updErr := updater(stack); updErr != nil {
			return fmt.Errorf("failed to update stack: %w", updErr)
		}

		model := newStackModel(&stack.StackDraft)
		model.ID = old.ID
		model.CreatedAt = old.CreatedAt
		model.UpdatedAt = time.Now()

		data, err := json.Marshal(model)
		if err != nil {
			return fmt.Errorf("failed to marshal stack: %w", err)
		}

		// Update the stack
		key := r.getStackKey(model.ID)
		if setErr := txn.Set(key, data); setErr != nil {
			return fmt.Errorf("failed to update stack: %w", setErr)
		}

		// Remove old indexes
		if rmErr := r.removeStackIndexes(txn, old); rmErr != nil {
			return fmt.Errorf("failed to remove stack indexes: %w", rmErr)
		}

		// Update indexes
		if crErr := r.createStackIndexes(txn, model); crErr != nil {
			return fmt.Errorf("failed to update stack indexes: %w", crErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update stack: %w", err)
	}

	return nil
}

// DeleteStack deletes a stack.
func (r *Repository) DeleteStack(_ context.Context, id uuid.UUID) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		// First, get the stack to remove indexes
		stack, err := r.getByID(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get stack before deletion: %w", err)
		}

		// Delete the stack
		key := r.getStackKey(id)
		if delErr := txn.Delete(key); delErr != nil && !errors.Is(delErr, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to delete stack: %w", delErr)
		}

		// Remove indexes
		if rmErr := r.removeStackIndexes(txn, stack); rmErr != nil {
			return fmt.Errorf("failed to remove stack indexes: %w", rmErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}

// ListStacks retrieves stacks based on filter criteria.
func (r *Repository) ListStacks(_ context.Context) ([]Stack, error) {
	var stacks []Stack

	err := r.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefix := []byte(prefixByID)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			if err := item.Value(func(val []byte) error {
				var stack stackModel
				if err := json.Unmarshal(val, &stack); err != nil {
					return fmt.Errorf("failed to unmarshal stack: %w", err)
				}

				stacks = append(stacks, *newStack(&stack))
				return nil
			}); err != nil {
				return fmt.Errorf("failed to unmarshal stack: %w", err)
			}
		}

		return nil
	})

	return stacks, fmt.Errorf("failed to list stacks: %w", err)
}

func (r *Repository) getByID(txn *badger.Txn, id uuid.UUID) (*stackModel, error) {
	var stack stackModel

	key := r.getStackKey(id)
	item, err := txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get stack: %w", err)
	}

	if valErr := item.Value(func(val []byte) error {
		return json.Unmarshal(val, &stack)
	}); valErr != nil {
		return nil, fmt.Errorf("failed to unmarshal stack: %w", valErr)
	}

	return &stack, nil
}

// getStackKey generates the key for storing a stack.
func (r *Repository) getStackKey(id uuid.UUID) []byte {
	return []byte(prefixByID + id.String())
}

// getStackNameKey generates the key for storing a stack name index.
func (r *Repository) getStackNameKey(name string) []byte {
	return []byte(prefixByName + name)
}

// createStackIndexes creates indexes for a stack.
func (r *Repository) createStackIndexes(txn *badger.Txn, stack *stackModel) error {
	// Name index
	nameKey := r.getStackNameKey(stack.Name)
	nameData, err := json.Marshal(stack.ID)
	if err != nil {
		return fmt.Errorf("failed to marshal stack ID: %w", err)
	}
	if setErr := txn.Set(nameKey, nameData); setErr != nil {
		return fmt.Errorf("failed to set name index: %w", setErr)
	}

	// Status index
	statusKey := []byte(prefixByStatus + string(stack.Status) + ":" + stack.ID.String())
	if setErr := txn.Set(statusKey, nameData); setErr != nil {
		return fmt.Errorf("failed to set status index: %w", setErr)
	}

	// Labels index
	for key, value := range stack.Labels {
		labelKey := []byte(prefixByLabel + key + ":" + value + ":" + stack.ID.String())
		if setErr := txn.Set(labelKey, nameData); setErr != nil {
			return fmt.Errorf("failed to set label index: %w", setErr)
		}
	}

	return nil
}

// removeStackIndexes removes indexes for a stack.
func (r *Repository) removeStackIndexes(txn *badger.Txn, stack *stackModel) error {
	// Name index
	nameKey := r.getStackNameKey(stack.Name)
	if err := txn.Delete(nameKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete name index: %w", err)
	}

	// Status index
	statusKey := []byte(prefixByStatus + string(stack.Status) + ":" + stack.ID.String())
	if err := txn.Delete(statusKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete status index: %w", err)
	}

	// Labels index
	for key, value := range stack.Labels {
		labelKey := []byte(prefixByLabel + key + ":" + value + ":" + stack.ID.String())
		if err := txn.Delete(labelKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to delete label index: %w", err)
		}
	}

	return nil
}
