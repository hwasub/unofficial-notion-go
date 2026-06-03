package notion

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
)

func pointerID(value any) string {
	if id, ok := value.(string); ok {
		return NormalizeID(id)
	}
	data, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	id, _ := data["id"].(string)
	return NormalizeID(id)
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func todoChecked(blk block) bool {
	if checked, ok := blk.Format["checked"].(bool); ok {
		return checked
	}
	value := strings.ToLower(strings.TrimSpace(plainText(blk.Properties["checked"])))
	return value == "yes" || value == "true" || value == "checked" || value == "1"
}

func notionPageHrefForInput(input RenderInput, pageID string) string {
	pageID = NormalizeID(pageID)
	if pageID == "" {
		return ""
	}
	storedPath, ok := input.PagePaths[pageID]
	if !ok {
		return ""
	}
	storedPath = strings.Trim(strings.TrimSpace(storedPath), "/")
	if storedPath != "" {
		if pathID := NormalizeID(storedPath); IsNormalizedID(pathID) {
			return notionPagePathHref(input.ResourceSlug, pathID)
		}
		return notionPagePathHref(input.ResourceSlug, pageID)
	}
	return notionPagePathHref(input.ResourceSlug, storedPath)
}

func notionPagePathHref(resourceSlug string, storedPath string) string {
	resourceSlug = strings.TrimSpace(resourceSlug)
	if resourceSlug == "" {
		return ""
	}
	href := "/" + url.PathEscape(resourceSlug)
	storedPath = strings.Trim(strings.TrimSpace(storedPath), "/")
	if storedPath != "" {
		href += "/" + url.PathEscape(storedPath)
	}
	return href
}

func notionPageIDFromLink(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "//") {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if parsed.IsAbs() {
		if !isNotionPageLinkHost(parsed.Hostname()) {
			return "", false
		}
	} else if !strings.HasPrefix(raw, "/") {
		return "", false
	}
	return notionPageIDFromPath(parsed.Path)
}

func isNotionPageLinkHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || host == "file.notion.so" {
		return false
	}
	return host == "notion.so" || host == "www.notion.so" || host == "notion.site" ||
		strings.HasSuffix(host, ".notion.so") || strings.HasSuffix(host, ".notion.site")
}

func notionPageIDFromPath(pathValue string) (string, bool) {
	segments := strings.Split(pathValue, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment, err := url.PathUnescape(strings.TrimSpace(segments[i]))
		if err != nil || segment == "" {
			continue
		}
		if pageID, ok := normalizedPageID(segment); ok {
			return pageID, true
		}
		if pageID, ok := notionPageIDSuffix(segment); ok {
			return pageID, true
		}
	}
	return "", false
}

func notionPageIDSuffix(segment string) (string, bool) {
	for i := len(segment) - 36; i >= 0; i-- {
		if pageID, ok := normalizedPageID(segment[i : i+36]); ok {
			return pageID, true
		}
	}
	compact := strings.ToLower(segment)
	for i := len(compact) - 32; i >= 0; i-- {
		candidate := compact[i : i+32]
		if isLowerHex(candidate) {
			if pageID, ok := normalizedPageID(candidate); ok {
				return pageID, true
			}
		}
	}
	return "", false
}

func isLowerHex(value string) bool {
	if value == "" {
		return false
	}
	for _, ch := range value {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func firstText(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return html.EscapeString(fallback)
	}
	return value
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func intValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	default:
		return 0
	}
}

func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func boolValue(value any) bool {
	v, ok := value.(bool)
	return ok && v
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if value, ok := value.(string); ok {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// NormalizeID canonicalizes a Notion block or page identifier to the hyphenated
// 8-4-4-4-12 UUID form. It lowercases the input, strips existing hyphens, and
// reinserts them; values that are not exactly 32 hex characters after stripping
// are returned trimmed but otherwise unchanged.
func NormalizeID(value string) string {
	value = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), "-", ""))
	if len(value) != 32 {
		return strings.TrimSpace(value)
	}
	return value[:8] + "-" + value[8:12] + "-" + value[12:16] + "-" + value[16:20] + "-" + value[20:]
}

// IsNormalizedID reports whether value is a canonical hyphenated 8-4-4-4-12
// UUID consisting solely of lowercase hex digits and hyphens in the expected
// positions, the form produced by NormalizeID.
func IsNormalizedID(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
				return false
			}
		}
	}
	return true
}

func normalizedPageID(value string) (string, bool) {
	id := NormalizeID(value)
	if !IsNormalizedID(id) {
		return "", false
	}
	return id, true
}
