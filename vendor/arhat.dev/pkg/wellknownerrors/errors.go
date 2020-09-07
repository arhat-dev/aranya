package wellknownerrors

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrAlreadyExists    = errors.New("already exists")
	ErrNotSupported     = errors.New("not supported")
	ErrInvalidOperation = errors.New("invalid operation")
)
