package notion

import (
	"fmt"
	"net/url"
	"strings"
)

func safeEmbedURL(blockType string, raw string) string {
	raw = safeURL(raw)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	// Upgrade http embeds to https. Passthrough provider branches return the
	// input URL verbatim; on an HTTPS page an http:// iframe is blocked as
	// mixed content, so the embed would silently fail to load.
	if parsed.Scheme == "http" {
		parsed.Scheme = "https"
		raw = parsed.String()
	}
	host := strings.ToLower(parsed.Hostname())
	cleanPath := strings.Trim(parsed.EscapedPath(), "/")
	segments := pathSegments(parsed.Path)
	switch {
	case host == "youtu.be":
		videoID := firstPathSegment(parsed.Path)
		if videoID != "" {
			return "https://www.youtube-nocookie.com/embed/" + url.PathEscape(videoID)
		}
	case host == "youtube.com" || host == "www.youtube.com" || host == "m.youtube.com":
		if embed := youtubeEmbedURL(parsed, cleanPath); embed != "" {
			return embed
		}
	case host == "vimeo.com" || host == "www.vimeo.com":
		videoID := firstPathSegment(parsed.Path)
		if videoID != "" {
			return "https://player.vimeo.com/video/" + url.PathEscape(videoID)
		}
	case host == "player.vimeo.com" && strings.HasPrefix(cleanPath, "video/"):
		return raw
	case host == "www.figma.com" || host == "figma.com":
		if cleanPath == "embed" || strings.HasPrefix(cleanPath, "embed/") {
			return raw
		}
		if blockType == "figma" || blockType == "embed" {
			return "https://www.figma.com/embed?embed_host=unofficial-notion-go&url=" + url.QueryEscape(raw)
		}
	case (host == "www.google.com" || host == "google.com") && strings.HasPrefix(cleanPath, "maps/embed"):
		return raw
	case host == "maps.google.com":
		if embed := googleMapsEmbedURL(parsed, segments, raw); embed != "" {
			return embed
		}
	case host == "www.google.com" || host == "google.com":
		if embed := googleMapsEmbedURL(parsed, segments, raw); embed != "" {
			return embed
		}
	case host == "open.spotify.com":
		if embed := spotifyEmbedURL(segments); embed != "" {
			return embed
		}
	case host == "www.loom.com" || host == "loom.com":
		if len(segments) >= 2 && (segments[0] == "share" || segments[0] == "embed") {
			return embedPath("https://www.loom.com/embed", segments[1])
		}
	case host == "fast.wistia.net":
		if blockType == "video" || blockType == "embed" {
			return wistiaEmbedURL(host, segments)
		}
	case host == "wistia.com" || strings.HasSuffix(host, ".wistia.com") || host == "wi.st" || strings.HasSuffix(host, ".wi.st"):
		if blockType == "video" || blockType == "embed" {
			return wistiaEmbedURL(host, segments)
		}
	case host == "www.tella.tv" || host == "tella.tv":
		if blockType == "video" || blockType == "embed" {
			return tellaEmbedURL(segments)
		}
	case host == "drive.google.com":
		if embed := googleDriveEmbedURL(parsed, host, segments, raw); embed != "" {
			return embed
		}
	case host == "docs.google.com":
		if embed := googleDriveEmbedURL(parsed, host, segments, raw); embed != "" {
			return embed
		}
	case host == "w.soundcloud.com" && cleanPath == "player":
		return raw
	case host == "soundcloud.com" || host == "www.soundcloud.com":
		if len(segments) >= 2 {
			return soundCloudEmbedURL(raw)
		}
	case host == "airtable.com" || host == "www.airtable.com":
		if embed := airtableEmbedURL(segments, raw); embed != "" {
			return embed
		}
	case host == "form.typeform.com" || strings.HasSuffix(host, ".typeform.com"):
		if embed := typeformEmbedURL(segments); embed != "" {
			return embed
		}
	case host == "codepen.io":
		if embed := codepenEmbedURL(segments, raw); embed != "" {
			return embed
		}
	case host == "replit.com" || host == "www.replit.com":
		return withEmbedParam(raw)
	case host == "miro.com" || host == "www.miro.com":
		if len(segments) >= 3 && segments[0] == "app" && segments[1] == "live-embed" {
			return raw
		}
		if len(segments) >= 3 && segments[0] == "app" && segments[1] == "board" {
			return embedPath("https://miro.com/app/live-embed", segments[2]) + "/"
		}
	case host == "excalidraw.com" || host == "www.excalidraw.com":
		if blockType == "excalidraw" || blockType == "embed" {
			return excalidrawEmbedURL(parsed)
		}
	case host == "gist.github.com":
		if len(segments) >= 2 {
			gistID := strings.TrimSuffix(segments[1], ".js")
			gistID = strings.TrimSuffix(gistID, ".pibb")
			if gistID != "" {
				return embedPath("https://gist.github.com", segments[0], gistID+".pibb")
			}
		}
	}
	return ""
}

func youtubeEmbedURL(parsed *url.URL, cleanPath string) string {
	if strings.HasPrefix(cleanPath, "embed/") {
		return "https://www.youtube-nocookie.com/" + cleanPath
	}
	if videoID := parsed.Query().Get("v"); videoID != "" {
		return "https://www.youtube-nocookie.com/embed/" + url.PathEscape(videoID)
	}
	return ""
}

func spotifyEmbedURL(segments []string) string {
	if len(segments) >= 3 && segments[0] == "embed" && segments[1] != "" && segments[2] != "" {
		return embedPath("https://open.spotify.com/embed", segments[1], segments[2])
	}
	if len(segments) >= 2 && segments[0] != "" && segments[0] != "embed" && segments[1] != "" {
		return embedPath("https://open.spotify.com/embed", segments[0], segments[1])
	}
	return ""
}

func airtableEmbedURL(segments []string, raw string) string {
	if len(segments) >= 2 && segments[0] == "embed" {
		return raw
	}
	if len(segments) >= 1 && strings.HasPrefix(segments[0], "shr") {
		return embedPath("https://airtable.com/embed", segments[0])
	}
	for _, segment := range segments {
		if strings.HasPrefix(segment, "shr") {
			return embedPath("https://airtable.com/embed", segment)
		}
	}
	return ""
}

func codepenEmbedURL(segments []string, raw string) string {
	if len(segments) >= 4 && segments[0] == "team" && segments[2] == "embed" {
		return raw
	}
	if len(segments) >= 3 && segments[1] == "embed" {
		return raw
	}
	if len(segments) >= 4 && segments[0] == "team" && segments[2] == "pen" {
		return embedPath("https://codepen.io", "team", segments[1], "embed", segments[3]) + "?default-tab=result"
	}
	if len(segments) >= 3 && segments[1] == "pen" {
		return embedPath("https://codepen.io", segments[0], "embed", segments[2]) + "?default-tab=result"
	}
	return ""
}

type tweetLinkInfo struct {
	User string
	ID   string
}

func parseTweetURL(raw string) (tweetLinkInfo, bool) {
	raw = safeURL(raw)
	if raw == "" || strings.HasPrefix(raw, "/") {
		return tweetLinkInfo{}, false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return tweetLinkInfo{}, false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	if host == "mobile.twitter.com" {
		host = "twitter.com"
	}
	if host != "twitter.com" && host != "x.com" {
		return tweetLinkInfo{}, false
	}
	segments := pathSegments(parsed.Path)
	for i, segment := range segments {
		if segment != "status" || i+1 >= len(segments) {
			continue
		}
		id := tweetID(segments[i+1])
		if id == "" {
			return tweetLinkInfo{}, false
		}
		user := ""
		if i > 0 && segments[i-1] != "web" {
			user = strings.TrimPrefix(segments[i-1], "@")
		}
		return tweetLinkInfo{User: user, ID: id}, true
	}
	return tweetLinkInfo{}, false
}

func tweetID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return value
}

func tweetHostLabel(raw string) string {
	host := linkHostLabel(raw)
	if host == "twitter.com" || host == "mobile.twitter.com" {
		return "x.com"
	}
	return host
}

func embedClass(blockType string, embedURL string) string {
	classes := []string{"notion-embed"}
	blockClass := sanitizeClassToken(blockType)
	if blockClass != "" {
		classes = append(classes, "notion-embed--"+blockClass)
	}
	if provider := embedProviderClass(embedURL); provider != "" && provider != blockClass && (blockClass == "" || blockClass == "embed" || blockClass == "video" || blockClass == "external_object_instance") {
		classes = append(classes, "notion-embed--"+provider)
	}
	return strings.Join(classes, " ")
}

func embedStyle(blk block) string {
	styles := make([]string, 0, 2)
	if width, ok := numberValue(blk.Format["block_width"]); ok && width > 0 && width <= 2000 {
		styles = append(styles, fmt.Sprintf("--notion-embed-width:%.0fpx", width))
	}
	if height, ok := numberValue(blk.Format["block_height"]); ok && height > 0 && height <= 2000 {
		styles = append(styles, fmt.Sprintf("--notion-embed-height:%.0fpx", height))
	}
	return strings.Join(styles, ";")
}

func embedProviderClass(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "www.youtube-nocookie.com":
		return "youtube"
	case host == "player.vimeo.com":
		return "vimeo"
	case host == "www.figma.com":
		return "figma"
	case host == "www.google.com" || host == "maps.google.com":
		return "maps"
	case host == "open.spotify.com":
		return "spotify"
	case host == "www.loom.com":
		return "loom"
	case host == "fast.wistia.net":
		return "wistia"
	case host == "www.tella.tv":
		return "tella"
	case host == "drive.google.com" || host == "docs.google.com":
		return "drive"
	case host == "w.soundcloud.com":
		return "soundcloud"
	case host == "airtable.com" || host == "www.airtable.com":
		return "airtable"
	case host == "form.typeform.com":
		return "typeform"
	case host == "codepen.io":
		return "codepen"
	case host == "replit.com" || host == "www.replit.com":
		return "replit"
	case host == "miro.com" || host == "www.miro.com":
		return "miro"
	case host == "excalidraw.com":
		return "excalidraw"
	case host == "gist.github.com":
		return "gist"
	default:
		return ""
	}
}

func googleMapsEmbedURL(parsed *url.URL, segments []string, raw string) string {
	host := strings.ToLower(parsed.Hostname())
	cleanPath := strings.Trim(parsed.EscapedPath(), "/")
	if (host == "www.google.com" || host == "google.com") && strings.HasPrefix(cleanPath, "maps/embed") {
		return raw
	}
	query := strings.TrimSpace(firstNonEmpty(parsed.Query().Get("q"), parsed.Query().Get("query")))
	if query == "" && len(segments) >= 3 && segments[0] == "maps" && (segments[1] == "place" || segments[1] == "search") {
		query = segments[2]
	}
	if query == "" && len(segments) >= 2 && segments[0] == "maps" && strings.HasPrefix(segments[1], "@") {
		query = strings.TrimPrefix(segments[1], "@")
	}
	if query == "" && host == "maps.google.com" && strings.TrimSpace(parsed.RawQuery) != "" {
		values := parsed.Query()
		values.Set("output", "embed")
		u := url.URL{Scheme: "https", Host: "maps.google.com", Path: parsed.Path, RawQuery: values.Encode()}
		return u.String()
	}
	if query == "" {
		return ""
	}
	values := url.Values{}
	values.Set("output", "embed")
	values.Set("q", query)
	return "https://www.google.com/maps?" + values.Encode()
}

func googleDriveEmbedURL(parsed *url.URL, host string, segments []string, raw string) string {
	if host == "drive.google.com" {
		if len(segments) >= 3 && segments[0] == "file" && segments[1] == "d" {
			return embedPath("https://drive.google.com/file/d", segments[2]) + "/preview"
		}
		if len(segments) >= 1 && segments[0] == "open" {
			if fileID := strings.TrimSpace(parsed.Query().Get("id")); fileID != "" {
				return embedPath("https://drive.google.com/file/d", fileID) + "/preview"
			}
		}
		if len(segments) >= 3 && segments[0] == "drive" && segments[1] == "folders" {
			return "https://drive.google.com/embeddedfolderview?id=" + url.QueryEscape(segments[2]) + "#list"
		}
		if len(segments) >= 1 && segments[0] == "embeddedfolderview" {
			return raw
		}
		return ""
	}
	if host != "docs.google.com" || len(segments) < 3 {
		return ""
	}
	switch segments[0] {
	case "document", "spreadsheets", "presentation", "drawings":
		if segments[1] == "d" {
			return embedPath("https://docs.google.com", segments[0], "d", segments[2]) + "/preview"
		}
	case "forms":
		if segments[1] == "d" {
			if len(segments) >= 4 && segments[2] == "e" {
				return embedPath("https://docs.google.com/forms/d/e", segments[3], "viewform") + "?embedded=true"
			}
			return embedPath("https://docs.google.com/forms/d", segments[2], "viewform") + "?embedded=true"
		}
	}
	return ""
}

func soundCloudEmbedURL(raw string) string {
	values := url.Values{}
	values.Set("auto_play", "false")
	values.Set("hide_related", "true")
	values.Set("show_comments", "false")
	values.Set("show_reposts", "false")
	values.Set("show_user", "true")
	values.Set("url", raw)
	values.Set("visual", "true")
	return "https://w.soundcloud.com/player/?" + values.Encode()
}

func typeformEmbedURL(segments []string) string {
	if len(segments) < 2 || segments[0] != "to" || segments[1] == "" {
		return ""
	}
	return embedPath("https://form.typeform.com/to", segments[1]) + "?typeform-embed=embed-widget"
}

func wistiaEmbedURL(host string, segments []string) string {
	if host == "fast.wistia.net" {
		if len(segments) >= 3 && segments[0] == "embed" && segments[1] == "iframe" {
			if mediaID := safeEmbedID(segments[2]); mediaID != "" {
				return embedPath("https://fast.wistia.net/embed/iframe", mediaID)
			}
		}
		return ""
	}
	for i, segment := range segments {
		if (segment == "medias" || segment == "embed") && i+1 < len(segments) {
			if mediaID := safeEmbedID(strings.TrimSuffix(segments[i+1], ".js")); mediaID != "" {
				return embedPath("https://fast.wistia.net/embed/iframe", mediaID)
			}
		}
	}
	return ""
}

func tellaEmbedURL(segments []string) string {
	if len(segments) < 2 || segments[0] != "video" {
		return ""
	}
	mediaID := safeEmbedID(segments[1])
	if mediaID == "" {
		return ""
	}
	return embedPath("https://www.tella.tv/video", mediaID) + "/embed"
}

func excalidrawEmbedURL(parsed *url.URL) string {
	out := *parsed
	out.Scheme = "https"
	out.Host = "excalidraw.com"
	out.User = nil
	if out.Path == "" {
		out.Path = "/"
	}
	return out.String()
}

func safeEmbedID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 {
		return ""
	}
	for _, ch := range value {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' {
			continue
		}
		return ""
	}
	return value
}

func withEmbedParam(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parsed.Fragment = ""
	values := parsed.Query()
	values.Set("embed", "true")
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func embedPath(base string, segments ...string) string {
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return ""
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return strings.TrimRight(base, "/") + "/" + strings.Join(escaped, "/")
}
