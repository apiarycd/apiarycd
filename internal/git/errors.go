package git

import "errors"

var (
	ErrRepositoryNotFound      = errors.New("repository not found")
	ErrCloneFailed             = errors.New("failed to clone repository")
	ErrPullFailed              = errors.New("failed to pull repository")
	ErrBranchNotFound          = errors.New("branch not found")
	ErrTagNotFound             = errors.New("tag not found")
	ErrFileNotFound            = errors.New("file not found")
	ErrInvalidRepository       = errors.New("invalid repository")
	ErrAuthenticationFailed    = errors.New("authentication failed")
	ErrRepositoryAlreadyExists = errors.New("repository already exists")
)
