package repositories

import "errors"

var (
	ErrValidationFailed = errors.New("validation failed")
	ErrBranchNotFound   = errors.New("branch not found")
)
