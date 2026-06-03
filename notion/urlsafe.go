package notion

import (
	"net/url"
	"path"
	"strings"
)

func safeButtonURL(raw string) string {
	href := safeURL(raw)
	if href == "" || isNotionAssetURL(href) {
		return ""
	}
	return href
}

func assetURL(input RenderInput, blockID string, role string) string {
	normalized := NormalizeID(blockID)
	keys := []string{
		normalized + ":" + role,
		strings.ReplaceAll(normalized, "-", "") + ":" + role,
	}
	if role == "cover" {
		keys = append(keys, normalized, strings.ReplaceAll(normalized, "-", ""))
	}
	for _, key := range keys {
		if value := input.AssetURLs[key]; strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func assetSourceURL(input RenderInput, blockID string, source string) string {
	return firstNonEmpty(assetAliasURL(input, blockID), assetAliasURL(input, source))
}

func resolvedAssetURL(input RenderInput, rm recordMap, blockID string, role string, source string) string {
	if local := safeURL(assetURL(input, blockID, role)); local != "" {
		return local
	}
	if local := safeURL(assetAliasURL(input, source)); local != "" {
		return local
	}
	keys := assetBlockKeys(blockID, role)
	keys = append(keys, source)
	return unsafeNotionAssetURL(input, rm, keys...)
}

func resolvedAssetSourceURL(input RenderInput, rm recordMap, blockID string, source string) string {
	if local := safeURL(assetSourceURL(input, blockID, source)); local != "" {
		return local
	}
	keys := assetBlockKeys(blockID, "")
	keys = append(keys, source)
	return unsafeNotionAssetURL(input, rm, keys...)
}

func resolvedAssetAliasURL(input RenderInput, rm recordMap, keys ...string) string {
	for _, key := range keys {
		if local := safeURL(assetAliasURL(input, key)); local != "" {
			return local
		}
	}
	return unsafeNotionAssetURL(input, rm, keys...)
}

func assetAliasURL(input RenderInput, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if value := input.AssetURLs[key]; strings.TrimSpace(value) != "" {
		return value
	}
	alias := assetCanonicalSourceAlias(key)
	if alias == "" || alias == key {
		return ""
	}
	return input.AssetURLs[alias]
}

func assetCanonicalSourceAlias(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	return u.String()
}

func assetBlockKeys(blockID string, role string) []string {
	normalized := NormalizeID(blockID)
	compact := strings.ReplaceAll(normalized, "-", "")
	keys := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			keys = append(keys, value)
		}
	}
	if role != "" {
		add(normalized + ":" + role)
		add(compact + ":" + role)
	}
	if role == "" || role == "cover" {
		add(normalized)
		add(compact)
	}
	return keys
}

func unsafeNotionAssetURL(input RenderInput, rm recordMap, keys ...string) string {
	if !input.UnsafeRenderNotionSignedURLs {
		return ""
	}
	aliases := assetLookupAliases(keys...)
	for _, key := range aliases {
		if src := safeNotionHostedURL(rm.SignedURLs[key]); src != "" {
			return src
		}
	}
	for _, key := range aliases {
		if src := safeNotionHostedURL(key); src != "" {
			return src
		}
	}
	return ""
}

func assetLookupAliases(keys ...string) []string {
	out := []string{}
	seen := map[string]struct{}{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, key := range keys {
		add(key)
		add(assetCanonicalSourceAlias(key))
	}
	return out
}

func safeNotionHostedURL(raw string) string {
	src := safeURL(raw)
	if src == "" || !isNotionHostedURL(src) {
		return ""
	}
	return src
}

func safeExternalImageURL(raw string) string {
	src := safeURL(raw)
	if src == "" || isNotionHostedURL(src) {
		return ""
	}
	parsed, err := url.Parse(src)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return ""
	}
	return src
}

func isNotionHostedURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "notion.so" || host == "www.notion.so" || host == "notion.site" ||
		strings.HasSuffix(host, ".notion.so") || strings.HasSuffix(host, ".notion.site") ||
		host == "secure.notion-static.com" || host == "prod-files-secure.s3.us-west-2.amazonaws.com" ||
		host == "s3-us-west-2.amazonaws.com" ||
		host == "s3.us-west-2.amazonaws.com" || host == "file.notion.so"
}

func isNotionAssetURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return host == "secure.notion-static.com" ||
		host == "prod-files-secure.s3.us-west-2.amazonaws.com" ||
		host == "s3-us-west-2.amazonaws.com" ||
		host == "s3.us-west-2.amazonaws.com" ||
		host == "file.notion.so"
}

func pathSegments(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, "/")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func firstPathSegment(value string) string {
	for _, part := range strings.Split(value, "/") {
		if part != "" {
			return part
		}
	}
	return ""
}

func linkHostLabel(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	return host
}

func isLocalPDFURL(raw string) bool {
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	return strings.HasSuffix(strings.ToLower(parsed.Path), ".pdf")
}

func pdfFilename(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "Open PDF"
	}
	name := path.Base(parsed.Path)
	if name == "." || name == "/" || name == "" {
		return "Open PDF"
	}
	if decoded, err := url.PathUnescape(name); err == nil && decoded != "" {
		name = decoded
	}
	return name
}

func safeRichTextLinkURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return "https://www.notion.so" + raw
	}
	if isNotionAssetURL(raw) {
		return ""
	}
	return safeURL(raw)
}

func safeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Browsers treat backslashes as forward slashes and strip ASCII control
	// characters (including TAB/CR/LF) before parsing a URL. Either trick can
	// smuggle a protocol-relative or cross-origin authority past the checks
	// below — e.g. "/\\evil.com" or "/<TAB>/evil.com" normalize to
	// "//evil.com", and "https:\\evil.com" normalizes to "https://evil.com".
	// Reject both outright.
	if strings.ContainsRune(raw, '\\') || strings.IndexFunc(raw, isURLControlChar) >= 0 {
		return ""
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		if u.Host == "" {
			// Reject opaque "scheme:opaque" forms with no authority; a browser
			// can still resolve these to a cross-origin host.
			return ""
		}
		return u.String()
	default:
		return ""
	}
}

func isURLControlChar(r rune) bool {
	return r < 0x20 || r == 0x7f
}
