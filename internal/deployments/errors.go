package deployments

import "errors"

var (
	ErrValidationFailed = errors.New("validation failed")
	ErrNotFound         = errors.New("deployment not found")
	ErrNotAllowed       = errors.New("operation not allowed")
)
