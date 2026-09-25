package utils

import (
	"errors"
	"fmt"
)

// Sentinel errors let services signal intent without importing net/http. Handlers map
// them to status codes in one place.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource conflict")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation failed")
)

// DomainError carries a sentinel kind, a message safe to show the caller, and optional
// field errors. The wrapped cause stays server-side.
type DomainError struct {
	Kind    error
	Message string
	Fields  map[string]string
	cause   error
}

func (e *DomainError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

func (e *DomainError) Unwrap() error { return e.Kind }

func (e *DomainError) Cause() error { return e.cause }

func NewDomainError(kind error, message string) *DomainError {
	return &DomainError{Kind: kind, Message: message}
}

func NewFieldError(message string, fields map[string]string) *DomainError {
	return &DomainError{Kind: ErrValidation, Message: message, Fields: fields}
}

func WrapDomainError(kind error, message string, cause error) *DomainError {
	return &DomainError{Kind: kind, Message: message, cause: cause}
}

func NotFound(resource string) *DomainError {
	return NewDomainError(ErrNotFound, resource+" not found")
}

func Conflict(message string) *DomainError {
	return NewDomainError(ErrConflict, message)
}

func Unauthorized(message string) *DomainError {
	return NewDomainError(ErrUnauthorized, message)
}

// AsDomainError extracts a *DomainError from an error chain.
func AsDomainError(err error) (*DomainError, bool) {
	var domainErr *DomainError
	if errors.As(err, &domainErr) {
		return domainErr, true
	}
	return nil, false
}
