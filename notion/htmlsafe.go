package notion

import "html"

// escapeText escapes a plain-text string for safe embedding inside HTML text
// content or a double-quoted attribute value. It is the single, named sink for
// text-to-HTML escaping; routing every dynamic plain-text value through it keeps
// the escaping structural and easy to audit.
func escapeText(s string) string {
	return html.EscapeString(s)
}

// attr renders a single HTML attribute, escaping the value. It returns the
// attribute with a leading space (e.g. ` name="value"`) so it can be appended
// directly to an open tag without the caller managing separators.
//
// attr only HTML-escapes the value; it does NOT validate URL schemes. For
// href/src attributes the value must already be sanitized (e.g. via safeURL),
// or use safeAttrURL, which pairs the safe-url check with escaping.
func attr(name, value string) string {
	return ` ` + name + `="` + html.EscapeString(value) + `"`
}

// safeAttrURL sanitizes a raw URL with safeURL and escapes the result for use as
// an HTML attribute value (typically href/src). It returns the empty string when
// safeURL rejects the URL, so callers can guard on emptiness before emitting an
// attribute. This centralizes the safe-url + escape pairing so the two steps
// cannot drift apart at a call site.
func safeAttrURL(raw string) string {
	safe := safeURL(raw)
	if safe == "" {
		return ""
	}
	return html.EscapeString(safe)
}
