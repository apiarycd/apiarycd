package deployments

import "errors"

var (
	ErrNotFound   = errors.New("deployment not found")
	ErrNotAllowed = errors.New("operation not allowed")
)
