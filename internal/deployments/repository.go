package deployments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

const (
	prefix = "deployment:"

	prefixByID     = prefix + "id:"
	prefixByStack  = prefix + "stack:"
	prefixByStatus = prefix + "status:"
	prefixByEnv    = prefix + "env:"
)

// Repository implements the DeploymentRepository interface.
type Repository struct {
	db *badger.DB
}

func NewRepository(db *badger.DB) *Repository {
	return &Repository{
		db: db,
	}
}

// Create creates a new deployment.
func (r *Repository) Create(_ context.Context, deployment *DeploymentDraft) error {
	model := newDeploymentModel(deployment)

	// Serialize the deployment
	data, err := json.Marshal(model)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	err = r.db.Update(func(txn *badger.Txn) error {
		// Store the deployment
		key := r.getKey(model.ID)
		if setErr := txn.Set(key, data); setErr != nil {
			return fmt.Errorf("failed to store deployment: %w", setErr)
		}

		// Create indexes
		if crErr := r.createIndexes(txn, model); crErr != nil {
			return fmt.Errorf("failed to create deployment indexes: %w", crErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	return nil
}

// GetByID retrieves a deployment by its ID.
func (r *Repository) GetByID(_ context.Context, id uuid.UUID) (*Deployment, error) {
	var deployment *deploymentModel

	err := r.db.View(func(txn *badger.Txn) error {
		found, err := r.getByID(txn, id)
		if err == nil {
			deployment = found
		}

		return err
	})

	return newDeployment(deployment), err
}

// GetLatestByStack retrieves the latest deployment for a stack.
func (r *Repository) GetLatestByStack(_ context.Context, stackID uuid.UUID) (*Deployment, error) {
	var latest *deploymentModel

	err := r.db.View(func(txn *badger.Txn) error {
		prefix := r.getStackPrefix(stackID)
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true // Get the latest (most recent) first

		it := txn.NewIterator(opts)
		defer it.Close()

		if it.Seek(prefix); !it.ValidForPrefix(prefix) {
			return nil
		}

		item := it.Item()

		var deploymentID uuid.UUID
		if err := item.Value(func(val []byte) error { return json.Unmarshal(val, &deploymentID) }); err != nil {
			return fmt.Errorf("failed to unmarshal deployment ID: %w", err)
		}

		found, err := r.getByID(txn, deploymentID)
		if err != nil {
			return fmt.Errorf("failed to get deployment by ID: %w", err)
		}
		latest = found

		return nil
	})

	if latest == nil {
		return nil, fmt.Errorf("%w for stack: %s", ErrNotFound, stackID.String())
	}

	return newDeployment(latest), err
}

// Update updates an existing deployment.
func (r *Repository) Update(_ context.Context, id uuid.UUID, updater func(*Deployment) error) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		old, err := r.getByID(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get deployment before update: %w", err)
		}

		deployment := newDeployment(old)

		if updErr := updater(deployment); updErr != nil {
			return fmt.Errorf("failed to update deployment: %w", updErr)
		}

		model := newDeploymentModel(&deployment.DeploymentDraft)
		model.ID = old.ID
		model.CreatedAt = old.CreatedAt
		model.UpdatedAt = time.Now()

		// Serialize the deployment
		data, err := json.Marshal(model)
		if err != nil {
			return fmt.Errorf("failed to marshal deployment: %w", err)
		}

		// Update the deployment
		key := r.getKey(deployment.ID)
		if setErr := txn.Set(key, data); setErr != nil {
			return fmt.Errorf("failed to update deployment: %w", setErr)
		}

		// Remove old indexes
		if rmErr := r.removeIndexes(txn, old); rmErr != nil {
			return fmt.Errorf("failed to remove deployment indexes: %w", rmErr)
		}

		// Update indexes
		if crErr := r.createIndexes(txn, model); crErr != nil {
			return fmt.Errorf("failed to update deployment indexes: %w", crErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update deployment: %w", err)
	}

	return nil
}

// Delete deletes a deployment.
func (r *Repository) Delete(_ context.Context, id uuid.UUID) error {
	err := r.db.Update(func(txn *badger.Txn) error {
		// First, get the deployment to remove indexes
		deployment, err := r.getByID(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get deployment before deletion: %w", err)
		}

		// Delete the deployment
		key := r.getKey(id)
		if delErr := txn.Delete(key); delErr != nil && !errors.Is(delErr, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to delete deployment: %w", delErr)
		}

		// Remove indexes
		if rmErr := r.removeIndexes(txn, deployment); rmErr != nil {
			return fmt.Errorf("failed to remove deployment indexes: %w", rmErr)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	return nil
}

// List retrieves deployments based on filter criteria.
func (r *Repository) List(_ context.Context) ([]Deployment, error) {
	var deployments []Deployment

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(prefixByID)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			if err := item.Value(func(val []byte) error {
				var deployment deploymentModel
				if err := json.Unmarshal(val, &deployment); err != nil {
					return fmt.Errorf("failed to unmarshal deployment: %w", err)
				}

				deployments = append(deployments, *newDeployment(&deployment))

				return nil
			}); err != nil {
				return fmt.Errorf("failed to unmarshal deployment: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return deployments, fmt.Errorf("failed to list deployments: %w", err)
	}

	return deployments, nil
}

func (r *Repository) ListByStack(_ context.Context, stackID uuid.UUID) ([]Deployment, error) {
	var deployments []Deployment

	err := r.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 10

		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := r.getStackPrefix(stackID)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()

			if err := item.Value(func(val []byte) error {
				var deployment deploymentModel
				if err := json.Unmarshal(val, &deployment); err != nil {
					return fmt.Errorf("failed to unmarshal deployment: %w", err)
				}

				deployments = append(deployments, *newDeployment(&deployment))

				return nil
			}); err != nil {
				return fmt.Errorf("failed to unmarshal deployment: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return deployments, fmt.Errorf("failed to list deployments: %w", err)
	}

	return deployments, nil
}

func (r *Repository) getByID(txn *badger.Txn, id uuid.UUID) (*deploymentModel, error) {
	key := r.getKey(id)
	item, err := txn.Get(key)
	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	var deployment deploymentModel
	if valErr := item.Value(func(val []byte) error { return json.Unmarshal(val, &deployment) }); valErr != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", valErr)
	}

	return &deployment, nil
}

// getKey generates the key for storing a deployment.
func (r *Repository) getKey(id uuid.UUID) []byte {
	return []byte(prefixByID + id.String())
}

// getStackPrefix generates the prefix for stack-specific deployments.
func (r *Repository) getStackPrefix(stackID uuid.UUID) []byte {
	return []byte(prefixByStack + stackID.String() + ":")
}

// createIndexes creates indexes for a deployment.
func (r *Repository) createIndexes(txn *badger.Txn, deployment *deploymentModel) error {
	// Stack ID index
	stackKey := []byte(
		prefixByStack + deployment.StackID.String() + ":" + strconv.FormatInt(deployment.CreatedAt.UnixNano(), 10),
	)
	stackData, err := json.Marshal(deployment.ID)
	if err != nil {
		return fmt.Errorf("failed to marshal deployment ID: %w", err)
	}
	if setErr := txn.Set(stackKey, stackData); setErr != nil {
		return fmt.Errorf("failed to set stack index: %w", setErr)
	}

	// Status index
	statusKey := []byte(prefixByStatus + string(deployment.Status) + ":" + deployment.ID.String())
	if setErr := txn.Set(statusKey, stackData); setErr != nil {
		return fmt.Errorf("failed to set status index: %w", setErr)
	}

	// Environment index
	envKey := []byte(prefixByEnv + deployment.Environment + ":" + deployment.ID.String())
	if setErr := txn.Set(envKey, stackData); setErr != nil {
		return fmt.Errorf("failed to set environment index: %w", setErr)
	}

	return nil
}

// removeIndexes removes indexes for a deployment.
func (r *Repository) removeIndexes(txn *badger.Txn, deployment *deploymentModel) error {
	// Stack ID index
	stackKey := []byte(
		prefixByStack + deployment.StackID.String() + ":" + strconv.FormatInt(deployment.CreatedAt.UnixNano(), 10),
	)
	if err := txn.Delete(stackKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete stack index: %w", err)
	}

	// Status index
	statusKey := []byte(prefixByStatus + string(deployment.Status) + ":" + deployment.ID.String())
	if err := txn.Delete(statusKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete status index: %w", err)
	}

	// Environment index
	envKey := []byte(prefixByEnv + deployment.Environment + ":" + deployment.ID.String())
	if err := txn.Delete(envKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
		return fmt.Errorf("failed to delete environment index: %w", err)
	}

	return nil
}
