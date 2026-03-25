package immutables

import "errors"

var (
	ErrInvalidComposeSectionType = errors.New("compose section has invalid type")
	ErrResourceNameCollision     = errors.New("resource name collision")
)
