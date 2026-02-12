// ABOUTME: Error types for the Zenfra API client.
// ABOUTME: Provides structured errors for HTTP status codes and helper functions for error type checking.

package zenfraclient

import (
	"errors"
	"fmt"
)

// APIError is the base error type returned by the Zenfra API.
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("zenfra API error (status %d, request_id %s): %s", e.StatusCode, e.RequestID, e.Message)
	}
	return fmt.Sprintf("zenfra API error (status %d): %s", e.StatusCode, e.Message)
}

// NotFoundError indicates the requested resource was not found (HTTP 404).
type NotFoundError struct {
	APIError
}

// ConflictError indicates a conflict with existing state (HTTP 409).
type ConflictError struct {
	APIError
}

// UnauthorizedError indicates missing or invalid authentication (HTTP 401).
type UnauthorizedError struct {
	APIError
}

// ForbiddenError indicates insufficient permissions (HTTP 403).
type ForbiddenError struct {
	APIError
}

// ValidationError indicates invalid request payload (HTTP 400/422).
type ValidationError struct {
	APIError
	Fields map[string]string `json:"fields,omitempty"`
}

// IsNotFound returns true if the error is a NotFoundError.
func IsNotFound(err error) bool {
	var nfe *NotFoundError
	return errors.As(err, &nfe)
}

// IsConflict returns true if the error is a ConflictError.
func IsConflict(err error) bool {
	var ce *ConflictError
	return errors.As(err, &ce)
}

// IsUnauthorized returns true if the error is an UnauthorizedError.
func IsUnauthorized(err error) bool {
	var ue *UnauthorizedError
	return errors.As(err, &ue)
}

// IsForbidden returns true if the error is a ForbiddenError.
func IsForbidden(err error) bool {
	var fe *ForbiddenError
	return errors.As(err, &fe)
}
