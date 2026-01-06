package stacks

// CreateRequest represents the request payload for creating a stack.
type CreateRequest struct {
	Name        string            `json:"name"                validate:"required,min=1,max=100"`
	Description string            `json:"description"         validate:"max=500"`
	GitURL      string            `json:"git_url"             validate:"required,url"`
	GitBranch   string            `json:"git_branch"          validate:"required,min=1,max=100"`
	ComposePath string            `json:"compose_path"        validate:"required,min=1,max=255"`
	Variables   map[string]string `json:"variables,omitempty"`
	AutoDeploy  bool              `json:"auto_deploy"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// UpdateRequest represents the request payload for updating a stack.
type UpdateRequest struct {
	Description *string            `json:"description,omitempty"  validate:"omitempty,max=500"`
	GitURL      *string            `json:"git_url,omitempty"      validate:"omitempty,url"`
	GitBranch   *string            `json:"git_branch,omitempty"   validate:"omitempty,min=1,max=100"`
	ComposePath *string            `json:"compose_path,omitempty" validate:"omitempty,min=1,max=255"`
	Variables   *map[string]string `json:"variables,omitempty"`
	AutoDeploy  *bool              `json:"auto_deploy,omitempty"`
	Labels      *map[string]string `json:"labels,omitempty"`
}
