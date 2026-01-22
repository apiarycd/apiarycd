package stacks

import (
	"path/filepath"

	"github.com/google/uuid"
)

// pathBuilder implements RepositoryPathBuilder interface.
type pathBuilder struct {
	basePath string
}

// NewPathBuilder creates a new RepositoryPathBuilder.
func NewPathBuilder(basePath string) RepositoryPathBuilder {
	return &pathBuilder{basePath: basePath}
}

// BuildPath builds the repository path for a stack.
func (p *pathBuilder) BuildPath(stackID uuid.UUID) string {
	return filepath.Join(p.basePath, "repositories", stackID.String())
}
