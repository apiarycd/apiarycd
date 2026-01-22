package stacks

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/apiarycd/apiarycd/internal/storage"
	"github.com/apiarycd/apiarycd/pkg/badgerfx"
	"github.com/google/uuid"
)

type Status string

const (
	prefix = "stack:"

	prefixByID     = prefix + "id:"
	prefixByName   = prefix + "name:"
	prefixByStatus = prefix + "status:"
	prefixByLabel  = prefix + "label:"
)

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusError    Status = "error"
)

type gitAuth struct {
	Type     string `json:"type"`     // Authentication type: "none", "https", "ssh"
	Username string `json:"username"` // Username for HTTPS auth
	Password string `json:"password"` // Password or token for HTTPS auth

	// SSH-specific fields
	PrivateKeyPath string `json:"private_key_path,omitempty"` // Path to SSH private key
	Passphrase     string `json:"passphrase,omitempty"`       // Passphrase for encrypted SSH key
}

// stackModel represents a GitOps stack configuration.
type stackModel struct {
	storage.BaseEntity

	// Basic Information
	Name        string `json:"name"`
	Description string `json:"description"`

	// Git Repository Information
	GitURL      string  `json:"git_url"`      // HTTPS or SSH URL
	GitBranch   string  `json:"git_branch"`   // Default branch to monitor
	GitAuth     gitAuth `json:"git_auth"`     // Git authentication
	ComposePath string  `json:"compose_path"` // Path to docker-compose.yml

	// Configuration
	Variables map[string]string `json:"variables"` // Default variables

	// Status
	Status     Status     `json:"status"`      // active, inactive, error
	LastSync   *time.Time `json:"last_sync"`   // Last successful sync
	LastDeploy *time.Time `json:"last_deploy"` // Last successful deployment

	// Metadata
	Labels map[string]string `json:"labels"` // Custom labels for filtering
}

func newStackModel(stack StackDraft) *stackModel {
	return &stackModel{
		BaseEntity: storage.BaseEntity{
			ID:        uuid.Must(uuid.NewV7()),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        stack.Name,
		Description: stack.Description,
		GitURL:      stack.GitURL,
		GitBranch:   stack.GitBranch,
		GitAuth: gitAuth{
			Type:           string(stack.GitAuth.Type),
			Username:       stack.GitAuth.Username,
			Password:       stack.GitAuth.Password,
			PrivateKeyPath: stack.GitAuth.PrivateKeyPath,
			Passphrase:     stack.GitAuth.Passphrase,
		},
		ComposePath: stack.ComposePath,
		Variables:   stack.Variables,
		Status:      StatusActive,
		LastSync:    nil,
		LastDeploy:  nil,
		Labels:      stack.Labels,
	}
}

func (s *stackModel) nameIndex() string {
	return prefixByName + s.Name
}

// MarshalStorage implements badgerfx.Entity.
func (s *stackModel) MarshalStorage() ([]byte, error) {
	data, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stack: %w", err)
	}

	return data, nil
}

// StorageIndexes implements badgerfx.Entity.
func (s *stackModel) StorageIndexes() []string {
	const fixedIndexCount = 2

	indexes := make([]string, 0, fixedIndexCount+len(s.Labels))

	// Name index
	indexes = append(indexes, s.nameIndex())

	// Status index
	indexes = append(indexes, prefixByStatus+string(s.Status)+":"+s.ID.String())

	// Labels index
	for key, value := range s.Labels {
		indexes = append(indexes, prefixByLabel+url.QueryEscape(key)+":"+url.QueryEscape(value)+":"+s.ID.String())
	}

	return indexes
}

// StorageKey implements badgerfx.Entity.
func (s *stackModel) StorageKey(id ...string) string {
	if len(id) > 0 {
		return prefixByID + id[0]
	}
	return prefixByID + s.ID.String()
}

// UnmarshalStorage implements badgerfx.Entity.
func (s *stackModel) UnmarshalStorage(data []byte) error {
	if err := json.Unmarshal(data, s); err != nil {
		return fmt.Errorf("failed to unmarshal stack: %w", err)
	}

	return nil
}

func (s *stackModel) update(stack StackUpdate) {
	s.Description = stack.Description
	s.GitURL = stack.GitURL
	s.GitBranch = stack.GitBranch
	s.GitAuth = gitAuth{
		Type:           string(stack.GitAuth.Type),
		Username:       stack.GitAuth.Username,
		Password:       stack.GitAuth.Password,
		PrivateKeyPath: stack.GitAuth.PrivateKeyPath,
		Passphrase:     stack.GitAuth.Passphrase,
	}
	s.ComposePath = stack.ComposePath
	s.Variables = stack.Variables
	s.Labels = stack.Labels

	s.Status = stack.Status
	s.LastSync = stack.LastSync
	s.LastDeploy = stack.LastDeploy

	s.UpdatedAt = time.Now()
}

func (s *stackModel) toDomain() *Stack {
	if s == nil {
		return nil
	}

	return &Stack{
		StackUpdate: StackUpdate{
			StackDraft: StackDraft{
				Name:        s.Name,
				Description: s.Description,
				GitURL:      s.GitURL,
				GitBranch:   s.GitBranch,
				GitAuth: GitAuth{
					Type:           GitAuthType(s.GitAuth.Type),
					Username:       s.GitAuth.Username,
					Password:       s.GitAuth.Password,
					PrivateKeyPath: s.GitAuth.PrivateKeyPath,
					Passphrase:     s.GitAuth.Passphrase,
				},
				ComposePath: s.ComposePath,
				Variables:   s.Variables,
				Labels:      s.Labels,
			},

			Status:     s.Status,
			LastSync:   s.LastSync,
			LastDeploy: s.LastDeploy,
		},

		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

var _ badgerfx.Entity = (*stackModel)(nil)
