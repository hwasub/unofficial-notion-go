// Package notionasset classifies Notion asset URLs without performing network I/O.
package notionasset

import (
	"net/url"
	"strings"
)

const Origin = "https://app.notion.com"

func IsHost(host string) bool {
	host = strings.ToLower(host)
	return host == "notion.so" || strings.HasSuffix(host, ".notion.so") || host == "notion.com" || strings.HasSuffix(host, ".notion.com")
}

func parsed(raw string) *url.URL {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return nil
	}
	return u
}

func legacy(u *url.URL) bool {
	switch strings.ToLower(u.Hostname()) {
	case "secure.notion-static.com", "prod-files-secure.s3.us-west-2.amazonaws.com", "prod-files-secure":
		return true
	case "s3.us-west-2.amazonaws.com", "s3-us-west-2.amazonaws.com":
		return strings.HasPrefix(u.Path, "/secure.notion-static.com/")
	}
	return false
}

func IsAsset(raw string) bool {
	if strings.HasPrefix(strings.TrimSpace(raw), "attachment:") {
		return true
	}
	u := parsed(raw)
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return legacy(u) || host == "file.notion.so" || host == "file.notion.com" || host == "img.notionusercontent.com" ||
		(IsHost(host) && (strings.HasPrefix(u.Path, "/image/") || strings.HasPrefix(u.Path, "/images/") || strings.HasPrefix(u.Path, "/icons/")))
}

func proxySource(u *url.URL) string {
	if IsHost(u.Hostname()) && strings.HasPrefix(u.EscapedPath(), "/image/") {
		s, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/image/"))
		if err == nil {
			return s
		}
	}
	return ""
}

func Signed(raw string) bool { return signed(raw, 0) }

func signed(raw string, depth int) bool {
	u := parsed(raw)
	if u == nil || !IsAsset(raw) {
		return false
	}
	for key := range u.Query() {
		k := strings.ToLower(key)
		if k == "signature" || k == "sig" || k == "tok" || k == "token" || k == "expirationtimestamp" || k == "exp" || k == "expires" || strings.HasPrefix(k, "x-amz-") {
			return true
		}
	}
	inner := proxySource(u)
	return inner != "" && (depth >= 5 || signed(inner, depth+1))
}

// Source recovers a stable asset identity. Opaque temporary URLs fail closed.
func Source(raw string) string { return source(strings.TrimSpace(raw), 0) }

func source(raw string, depth int) string {
	if depth > 5 || len(raw) > 32*1024 {
		return ""
	}
	if strings.HasPrefix(raw, "attachment:") {
		if strings.TrimPrefix(raw, "attachment:") != "" {
			return raw
		}
		return ""
	}
	if strings.HasPrefix(raw, "/images/") || strings.HasPrefix(raw, "/icons/") {
		return Origin + raw
	}
	u := parsed(raw)
	if u == nil || !IsAsset(raw) {
		return ""
	}
	if inner := proxySource(u); inner != "" {
		return source(inner, depth+1)
	}
	host := strings.ToLower(u.Hostname())
	if host == "file.notion.com" || host == "file.notion.so" {
		parts := strings.SplitN(strings.TrimPrefix(u.Path, "/"), "/", 5)
		if len(parts) == 5 && parts[0] == "f" && parts[1] == "f" && parts[2] != "" && parts[3] != "" && parts[4] != "" {
			return "attachment:" + parts[3] + ":" + parts[4]
		}
	}
	if host == "img.notionusercontent.com" {
		parts := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
		if len(parts) >= 3 && parts[0] == "s3" && parts[2] == "size" {
			decoded, err := url.PathUnescape(parts[1])
			file := strings.SplitN(decoded, "/", 4)
			if err == nil && len(file) == 4 && strings.HasPrefix(file[0], "prod-files-secure") && file[1] != "" && file[2] != "" && file[3] != "" {
				return "attachment:" + file[2] + ":" + file[3]
			}
		}
	}
	if legacy(u) {
		u.RawQuery, u.Fragment, u.ForceQuery = "", "", false
	} else if Signed(raw) {
		return ""
	}
	u.Scheme = "https"
	return u.String()
}

// DownloadURL accepts only direct HTTPS Notion asset URLs, never attachment IDs.
func DownloadURL(raw string) string {
	u := parsed(raw)
	if u == nil || u.Scheme != "https" || !IsAsset(raw) {
		return ""
	}
	if inner := proxySource(u); inner != "" && Source(inner) == "" {
		return ""
	}
	return u.String()
}

func SigningSource(raw string) string {
	s := Source(raw)
	if strings.HasPrefix(s, "attachment:") {
		return s
	}
	u := parsed(s)
	if u != nil && legacy(u) {
		return s
	}
	return ""
}

// ForStorage removes credentials only from recognized Notion asset URLs.
func ForStorage(raw string) string {
	if Signed(raw) {
		return Source(raw)
	}
	return raw
}
