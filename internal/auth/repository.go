package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// badgerUserRepository implements UserRepository using BadgerDB
type badgerUserRepository struct {
	db *badger.DB
}

// NewBadgerUserRepository creates a new BadgerDB-based UserRepository
func NewBadgerUserRepository(db *badger.DB) *badgerUserRepository {
	return &badgerUserRepository{db: db}
}

// GetByID retrieves a user by ID
func (r *badgerUserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	var user *userModel
	err := r.db.View(func(txn *badger.Txn) error {
		var err error
		user, err = r.get(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return user.toDomain(), nil
}

func (r *badgerUserRepository) GetByName(ctx context.Context, name string) (*User, error) {
	var user *userModel
	key := []byte("user:name:" + name)
	err := r.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return ErrUserNotFound
			}
			return fmt.Errorf("failed to get user: %w", err)
		}

		id := uuid.UUID{}
		if err := item.Value(func(val []byte) error {
			return json.Unmarshal(val, &id)
		}); err != nil {
			return fmt.Errorf("failed to unmarshal user: %w", err)
		}

		user, err = r.get(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return user.toDomain(), nil
}

func (r *badgerUserRepository) get(txn *badger.Txn, id uuid.UUID) (*userModel, error) {
	key := []byte("user:" + id.String())
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	user := new(userModel)
	if err := item.Value(func(val []byte) error {
		return user.FromBadgerValue(val)
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return user, nil
}

// Create creates a new user
func (r *badgerUserRepository) Create(ctx context.Context, user UserDraft, passwordHash string) error {
	model := newUserModel(user, passwordHash)

	return r.db.Update(func(txn *badger.Txn) error {
		// Check if user with this name already exists
		if _, err := r.GetByName(ctx, user.Name); err == nil {
			return ErrDuplicateUser
		}

		return r.write(txn, model)
	})
}

// Update updates an existing user
func (r *badgerUserRepository) Update(ctx context.Context, id uuid.UUID, updater func(*User) error) error {
	return r.db.Update(func(txn *badger.Txn) error {
		userModel, err := r.get(txn, id)
		if err != nil {
			return err
		}

		user := userModel.toDomain()
		if err := updater(user); err != nil {
			return err
		}

		userModel.update(user.UserBase)

		return r.write(txn, userModel)
	})
}

func (r *badgerUserRepository) write(txn *badger.Txn, user *userModel) error {
	key := user.ToBadgerKey()
	value, err := user.ToBadgerValue()
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}
	indexes := user.indexes()

	if err := txn.Set(key, value); err != nil {
		return fmt.Errorf("failed to set user: %w", err)
	}

	if len(indexes) == 0 {
		return nil
	}

	id, err := json.Marshal(user.ID)
	if err != nil {
		return fmt.Errorf("failed to marshal user ID: %w", err)
	}

	for _, index := range indexes {
		if err := txn.Set([]byte(index), id); err != nil {
			return fmt.Errorf("failed to set user index: %w", err)
		}
	}

	return nil
}

// Delete deletes a user
func (r *badgerUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	key := []byte("user:" + id.String())

	return r.db.Update(func(txn *badger.Txn) error {
		user, err := r.get(txn, id)
		if err != nil {
			return err
		}

		for _, index := range user.indexes() {
			if err := txn.Delete([]byte(index)); err != nil {
				return fmt.Errorf("failed to delete user index: %w", err)
			}
		}

		return txn.Delete(key)
	})
}

// badgerAPIKeyRepository implements APIKeyRepository using BadgerDB
type badgerAPIKeyRepository struct {
	db *badger.DB
}

// NewBadgerAPIKeyRepository creates a new BadgerDB-based APIKeyRepository
func NewBadgerAPIKeyRepository(db *badger.DB) *badgerAPIKeyRepository {
	return &badgerAPIKeyRepository{db: db}
}

// GetByID retrieves an API key by ID
func (r *badgerAPIKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*APIKey, error) {
	var apiKey *apiKeyModel
	err := r.db.View(func(txn *badger.Txn) error {
		var err error
		apiKey, err = r.get(txn, id)
		if err != nil {
			return fmt.Errorf("failed to get API key: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return apiKey.toDomain(), nil
}

// GetByUserID retrieves all API keys for a user
func (r *badgerAPIKeyRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]APIKey, error) {
	var apiKeys []APIKey
	err := r.db.View(func(txn *badger.Txn) error {
		iterator := txn.NewIterator(badger.DefaultIteratorOptions)
		defer iterator.Close()

		prefix := []byte("apikey:user:" + userID.String() + ":")
		for iterator.Seek(prefix); iterator.ValidForPrefix(prefix); iterator.Next() {
			item := iterator.Item()

			if err := item.Value(func(val []byte) error {
				id := uuid.UUID{}
				if err := json.Unmarshal(val, &id); err != nil {
					return fmt.Errorf("failed to unmarshal API key: %w", err)
				}

				apiKey, err := r.get(txn, id)
				if err != nil {
					return fmt.Errorf("failed to get API key: %w", err)
				}

				apiKeys = append(apiKeys, *apiKey.toDomain())
				return nil
			}); err != nil {
				return fmt.Errorf("failed to unmarshal API key: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return apiKeys, nil
}

func (r *badgerAPIKeyRepository) get(txn *badger.Txn, id uuid.UUID) (*apiKeyModel, error) {
	key := []byte("apikey:" + id.String())
	item, err := txn.Get(key)
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	apiKey := new(apiKeyModel)
	if err := item.Value(func(val []byte) error {
		return apiKey.FromBadgerValue(val)
	}); err != nil {
		return nil, err
	}

	return apiKey, nil
}

// Create creates a new API key
func (r *badgerAPIKeyRepository) Create(ctx context.Context, apiKey APIKeyDraft) error {
	model := newAPIKeyModel(apiKey)

	return r.db.Update(func(txn *badger.Txn) error {
		existing, err := r.get(txn, model.ID)
		if err != nil && !errors.Is(err, ErrAPIKeyNotFound) {
			return err
		}

		if existing != nil {
			return ErrAPIKeyAlreadyExists
		}

		return r.write(txn, model)
	})
}

// Update updates an existing API key
func (r *badgerAPIKeyRepository) Update(ctx context.Context, id uuid.UUID, updater func(*APIKey) error) error {
	return r.db.Update(func(txn *badger.Txn) error {
		model, err := r.get(txn, id)
		if err != nil {
			return err
		}

		apiKey := model.toDomain()
		if updErr := updater(apiKey); updErr != nil {
			return fmt.Errorf("failed to update API key: %w", updErr)
		}

		model.update(apiKey.APIKeyDraft)

		return r.write(txn, model)
	})
}

func (r *badgerAPIKeyRepository) write(txn *badger.Txn, user *apiKeyModel) error {
	key := user.ToBadgerKey()
	value, err := user.ToBadgerValue()
	if err != nil {
		return fmt.Errorf("failed to marshal API key: %w", err)
	}
	indexes := user.indexes()

	if err := txn.Set(key, value); err != nil {
		return fmt.Errorf("failed to set API key: %w", err)
	}

	if len(indexes) == 0 {
		return nil
	}

	id, err := json.Marshal(user.ID)
	if err != nil {
		return fmt.Errorf("failed to marshal API key ID: %w", err)
	}

	for _, index := range indexes {
		if err := txn.Set([]byte(index), id); err != nil {
			return fmt.Errorf("failed to set API key index: %w", err)
		}
	}

	return nil
}

// Delete deletes an API key
func (r *badgerAPIKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.Update(func(txn *badger.Txn) error {
		apiKey, err := r.get(txn, id)
		if err != nil {
			return err
		}

		for _, index := range apiKey.indexes() {
			if err := txn.Delete([]byte(index)); err != nil {
				return fmt.Errorf("failed to delete API key index: %w", err)
			}
		}

		return txn.Delete(apiKey.ToBadgerKey())
	})
}
