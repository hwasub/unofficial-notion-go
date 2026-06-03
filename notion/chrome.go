package notion

import "strings"

// ReaderChrome holds the parts of a rendered page separated for reader-style
// layouts: the page Body with its leading cover and icon removed, and the Cover
// and Icon HTML fragments (each empty when the page has none).
type ReaderChrome struct {
	Body  string
	Cover string
	Icon  string
}

// SplitReaderChrome pulls the leading cover and page-icon hero out of the HTML
// produced by RenderPage so callers can position them independently. When no
// cover or icon is present it returns the input unchanged as Body.
func SplitReaderChrome(renderedHTML string) ReaderChrome {
	parts := ReaderChrome{Body: renderedHTML}
	bodyStart := notionBodyOpenEnd(renderedHTML)
	if bodyStart < 0 {
		return parts
	}
	remaining := renderedHTML[bodyStart:]
	if strings.HasPrefix(remaining, `<figure class="notion-cover `) || strings.HasPrefix(remaining, `<figure class="notion-cover"`) {
		if end := strings.Index(remaining, `</figure>`); end >= 0 {
			end += len(`</figure>`)
			parts.Cover = remaining[:end]
			remaining = remaining[end:]
		}
	}
	if strings.HasPrefix(remaining, `<div class="notion-page-icon-hero"`) {
		if end := strings.Index(remaining, `</div>`); end >= 0 {
			end += len(`</div>`)
			parts.Icon = remaining[:end]
			remaining = remaining[end:]
		}
	}
	if parts.Cover == "" && parts.Icon == "" {
		return parts
	}
	parts.Body = renderedHTML[:bodyStart] + remaining
	return parts
}

func notionBodyOpenEnd(renderedHTML string) int {
	if !strings.HasPrefix(renderedHTML, `<div class="notion-body`) {
		return -1
	}
	end := strings.Index(renderedHTML, `>`)
	if end < 0 {
		return -1
	}
	return end + 1
}
