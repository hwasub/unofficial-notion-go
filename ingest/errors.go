package ingest

import (
	"errors"
	"fmt"
	"strings"

	"github.com/hwasub/unofficial-notion-go/notionapi"
)

// Stable machine-readable codes carried in HTTPError.Code. Callers can match
// on these instead of hardcoding wire strings. Limit and transport codes are
// shared with (and forwarded from) the notionapi package.
const (
	// ErrorCodeMaxBlocksExceeded marks a page block-count limit failure.
	ErrorCodeMaxBlocksExceeded = notionapi.ErrorCodeMaxBlocksExceeded
	// ErrorCodeMaxResponseBytesExceeded marks a response-size limit failure.
	ErrorCodeMaxResponseBytesExceeded = notionapi.ErrorCodeMaxResponseBytesExceeded
	// ErrorCodeUnexpectedContentType marks an upstream response that was not JSON.
	ErrorCodeUnexpectedContentType = notionapi.ErrorCodeUnexpectedContentType
	// ErrorCodeMalformedResponse marks an upstream response that exceeded
	// structural limits (nesting depth, array length).
	ErrorCodeMalformedResponse = notionapi.ErrorCodeMalformedResponse
	// ErrorCodeFetchTimeout marks a page fetch that exceeded the request timeout.
	ErrorCodeFetchTimeout = "notion_fetch_timeout"
	// ErrorCodeRateLimited marks an upstream 429 response.
	ErrorCodeRateLimited = "notion_rate_limited"
	// ErrorCodePageNotFound marks a page that does not exist or is not public.
	ErrorCodePageNotFound = "notion_page_not_found"
	// ErrorCodeUpstreamError marks any other upstream Notion failure.
	ErrorCodeUpstreamError = "notion_upstream_error"
	// ErrorCodeInternal marks an unclassified internal failure.
	ErrorCodeInternal = "internal_error"
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
