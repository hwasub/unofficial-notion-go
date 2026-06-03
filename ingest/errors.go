package ingest

import (
	"errors"
	"fmt"
	"strings"
)

// HTTPError is an ingest failure carrying an HTTP status code, a stable
// machine-readable code, a human-readable message, and an optional retry-after
// hint. It is the error type returned by FetchSnapshot.
type HTTPError struct {
	StatusCode   int    // HTTP status code to surface to the caller
	Code         string // stable machine-readable error code
	Message      string // human-readable error message
	RetryAfterMS int    // suggested retry delay in milliseconds, if any
}

// Error returns the error's code and message, falling back to a generic message
// when none is set. It is nil-safe and returns "" for a nil receiver.
func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "request failed"
	}
	if e.Code != "" {
		return fmt.Sprintf("%s: %s", e.Code, message)
	}
	return message
}

// NewHTTPError constructs an *HTTPError with the given message, HTTP status
// code, and machine-readable code.
func NewHTTPError(message string, statusCode int, code string) *HTTPError {
	return &HTTPError{Message: message, StatusCode: statusCode, Code: code}
}

// AsHTTPError reports whether err is, or wraps, an *HTTPError and returns it
// when found.
func AsHTTPError(err error) (*HTTPError, bool) {
	var target *HTTPError
	if errors.As(err, &target) {
		return target, true
	}
	return nil, false
}
