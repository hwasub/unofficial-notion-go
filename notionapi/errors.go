package notionapi

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Local error codes for limits enforced by this client.
const (
	// ErrorCodeMaxResponseBytesExceeded marks a response-size limit failure.
	ErrorCodeMaxResponseBytesExceeded = "max_response_bytes_exceeded"
	// ErrorCodeMaxBlocksExceeded marks a page block-count limit failure.
	ErrorCodeMaxBlocksExceeded = "max_blocks_exceeded"
	// ErrorCodeUnexpectedContentType marks a response whose Content-Type is not JSON.
	ErrorCodeUnexpectedContentType = "unexpected_content_type"
	// ErrorCodeMalformedResponse marks a decoded response that exceeds the
	// structural limits Fetch enforces (nesting depth, array length).
	ErrorCodeMalformedResponse = "malformed_response"
)

// HTTPError reports a non-2xx response from the Notion API or a local response
// size failure. Fetch returns it so callers can branch on the HTTP status and
// honor rate limits.
type HTTPError struct {
	StatusCode   int    // HTTP status code from upstream, or a synthetic status for local guards
	Code         string // optional machine-readable code for local client failures
	Message      string // parsed upstream error message, or a fallback
	RetryAfterMS int    // suggested retry delay in milliseconds, from Retry-After (0 if none)
}

// Error implements the error interface, formatting the status code and message.
// It is safe to call on a nil *HTTPError, which returns the empty string.
func (e *HTTPError) Error() string {
	if e == nil {
		return ""
	}
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = "request failed"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("notion upstream %d: %s", e.StatusCode, message)
	}
	return message
}

// RetryAfterToMS converts an HTTP Retry-After header value to milliseconds,
// relative to now. It accepts either a delay in seconds or an HTTP date, rounding
// fractional seconds up. It returns 0 when value is empty, unparseable, or refers
// to a time already in the past.
func RetryAfterToMS(value string, now time.Time) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
		return int(seconds*1000 + 0.999999)
	}
	if parsed, err := httpTimeParse(value); err == nil {
		delta := parsed.Sub(now)
		if delta < 0 {
			return 0
		}
		return int(delta / time.Millisecond)
	}
	return 0
}

func httpTimeParse(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC1123, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC1123Z, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.Parse(time.RFC850, value); err == nil {
		return parsed, nil
	}
	return time.Parse(time.ANSIC, value)
}
