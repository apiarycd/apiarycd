package repositories

import (
	"errors"
	"fmt"
)

var (
	ErrValidationFailed = errors.New("validation failed")
	ErrBranchNotFound   = errors.New("branch not found")
)

// gitCommandError is a structured error returned when a git command fails.
// It captures the subcommand, underlying error, and stderr output for
// programmatic inspection.
type gitCommandError struct {
	ExitCode   int
	SubCommand string
	Err        error
	Stderr     string
}

func (e *gitCommandError) Error() string {
	return fmt.Sprintf("git %s failed: %v: %s", e.SubCommand, e.Err, e.Stderr)
}

func (e *gitCommandError) Unwrap() error {
	return e.Err
}
