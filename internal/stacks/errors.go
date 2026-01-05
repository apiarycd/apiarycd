package stacks

import "errors"

var (
	ErrNotFound   = errors.New("stack not found")
	ErrConflict   = errors.New("stack already exists")
	ErrNotAllowed = errors.New("operation not allowed")
)
