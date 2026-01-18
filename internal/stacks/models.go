package stacks

import (
	"encoding/json"
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
	Username string `json:"username"`
	Password string `json:"password"`
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

func newStackModel(stack *StackDraft) *stackModel {
	if stack == nil {
		return nil
	}

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
			Username: stack.GitAuth.Username,
			Password: stack.GitAuth.Password,
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
	return json.Marshal(s)
}

// StorageIndexes implements badgerfx.Entity.
func (s *stackModel) StorageIndexes() []string {
	indexes := make([]string, 0, 3+len(s.Labels))

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
	return json.Unmarshal(data, s)
}

func (s *stackModel) toDomain() *Stack {
	if s == nil {
		return nil
	}

	return &Stack{
		StackDraft: StackDraft{
			Name:        s.Name,
			Description: s.Description,
			GitURL:      s.GitURL,
			GitBranch:   s.GitBranch,
			GitAuth: GitAuth{
				Username: s.GitAuth.Username,
				Password: s.GitAuth.Password,
			},
			ComposePath: s.ComposePath,
			Variables:   s.Variables,
			Labels:      s.Labels,
		},
		ID:        s.ID,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,

		Status:     s.Status,
		LastSync:   s.LastSync,
		LastDeploy: s.LastDeploy,
	}
}

var _ badgerfx.Entity = (*stackModel)(nil)
