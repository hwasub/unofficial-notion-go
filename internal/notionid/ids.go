// Package notionid normalizes Notion page and block identifiers across the
// several shapes Notion uses: hyphenated UUIDs, 32-character compact IDs,
// full page URLs, and pasted path fragments. It provides helpers to parse,
// hyphenate, and compact those identifiers into canonical forms.
package notionid

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var (
	compactIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
	pageIDPattern    = regexp.MustCompile(`[0-9a-fA-F]{32}|[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	apiCompactIDRe   = regexp.MustCompile(`\b([\da-f]{32})\b`)
	apiUUIDRe        = regexp.MustCompile(`\b([\da-f]{8}(?:-[\da-f]{4}){3}-[\da-f]{12})\b`)
)

// CompactID returns value as a lowercase 32-character ID with all hyphens
// removed. It returns the empty string when value is nil.
func CompactID(value any) string {
	if value == nil {
		return ""
	}
	return strings.ReplaceAll(strings.ToLower(fmt.Sprint(value)), "-", "")
}

// HyphenatePageID converts value into a hyphenated lowercase UUID. It returns
// the empty string when value is not a valid 32-character compact ID.
func HyphenatePageID(value any) string {
	id := CompactID(value)
	if !compactIDPattern.MatchString(id) {
		return ""
	}
	return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
}

// ParsePageID accepts UUIDs, compact IDs, Notion URLs, and pasted path
// fragments, returning a hyphenated lowercase UUID.
func ParsePageID(input any) string {
	value := strings.TrimSpace(fmt.Sprint(input))
	if value == "" || value == "<nil>" {
		return ""
	}
	if direct := HyphenatePageID(value); direct != "" {
		return direct
	}

	pathname := value
	if parsed, err := url.Parse(value); err == nil && parsed.Path != "" {
		pathname = parsed.Path
	}
	matches := pageIDPattern.FindAllString(pathname, -1)
	if len(matches) == 0 {
		return ""
	}
	return HyphenatePageID(matches[len(matches)-1])
}

// NormalizeBlockID returns value as a hyphenated lowercase UUID when it parses
// as one, and otherwise falls back to its plain string representation (the
// empty string when value is nil).
func NormalizeBlockID(value any) string {
	if id := HyphenatePageID(value); id != "" {
		return id
	}
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// IsNormalizedID reports whether value is already a hyphenated lowercase UUID,
// meaning it is unchanged by HyphenatePageID.
func IsNormalizedID(value string) bool {
	return HyphenatePageID(value) == value
}

// UUIDToID returns uuid with all hyphens removed, yielding a compact ID.
func UUIDToID(uuid string) string {
	return strings.ReplaceAll(uuid, "-", "")
}

// ParsePageIDForAPI extracts the first ID found in id (after stripping any
// query string) and returns it in hyphenated UUID form when uuid is true or in
// compact form otherwise. It returns the empty string when no ID is found.
func ParsePageIDForAPI(id string, uuid bool) string {
	id = strings.Split(id, "?")[0]
	if id == "" {
		return ""
	}
	if match := apiCompactIDRe.FindStringSubmatch(id); len(match) == 2 {
		if uuid {
			return HyphenatePageID(match[1])
		}
		return match[1]
	}
	if match := apiUUIDRe.FindStringSubmatch(id); len(match) == 2 {
		if uuid {
			return strings.ToLower(match[1])
		}
		return strings.ReplaceAll(strings.ToLower(match[1]), "-", "")
	}
	return ""
}
