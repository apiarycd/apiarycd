package repositories

import "errors"

var (
	ErrCloneFailed      = errors.New("failed to clone repository")
	ErrValidationFailed = errors.New("validation failed")
	ErrBranchNotFound   = errors.New("branch not found")
)
