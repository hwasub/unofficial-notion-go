package notion

import (
	"strings"
	"testing"
)

func TestSafeEmbedURLUpgradesHTTPToHTTPS(t *testing.T) {
	// Passthrough providers must not emit http:// iframes (blocked as mixed
	// content on HTTPS pages).
	got := safeEmbedURL("video", "http://player.vimeo.com/video/123")
	if got != "https://player.vimeo.com/video/123" {
		t.Fatalf("safeEmbedURL http vimeo = %q, want https passthrough", got)
	}
	if https := safeEmbedURL("video", "https://player.vimeo.com/video/123"); https != "https://player.vimeo.com/video/123" {
		t.Fatalf("https vimeo changed: %q", https)
	}
}

func TestSafeEmbedURLSpotifyRejectsTruncatedEmbed(t *testing.T) {
	if got := safeEmbedURL("embed", "https://open.spotify.com/embed/track"); got != "" {
		t.Fatalf("truncated spotify embed = %q, want empty (no double /embed/)", got)
	}
	if got := safeEmbedURL("embed", "https://open.spotify.com/track/abc123"); !strings.Contains(got, "/embed/track/abc123") {
		t.Fatalf("valid spotify track = %q, want /embed/track/abc123", got)
	}
}

func TestSafeEmbedURLFigmaTightPrefix(t *testing.T) {
	// A non-embed path that merely starts with "embed" must be wrapped, not
	// passed through as if it were already an embed URL.
	got := safeEmbedURL("figma", "https://www.figma.com/embedded/abc")
	if !strings.Contains(got, "figma.com/embed?embed_host=unofficial-notion-go") {
		t.Fatalf("figma /embedded/ = %q, want wrapped embed URL", got)
	}
	if raw := safeEmbedURL("figma", "https://www.figma.com/embed?url=x"); raw != "https://www.figma.com/embed?url=x" {
		t.Fatalf("real figma embed should pass through unchanged, got %q", raw)
	}
}

func TestApplyDecorationsColorForegroundVsBackground(t *testing.T) {
	fg := applyDecorations("x", "x", []any{[]any{"h", "blue"}}, nil)
	if !strings.Contains(fg, `<span class="notion-color--blue">`) || strings.Contains(fg, "<mark") {
		t.Fatalf("foreground color = %q, want a colored span (not a highlight mark)", fg)
	}
	bg := applyDecorations("x", "x", []any{[]any{"h", "blue_background"}}, nil)
	if !strings.Contains(bg, `<mark class="notion-highlight notion-color--blue-background">`) {
		t.Fatalf("background color = %q, want a highlight mark", bg)
	}
}

func TestNotionLinkFragmentAnchor(t *testing.T) {
	block := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	page := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"in-page heading link", "/" + page + "#" + block, "notion-" + block},
		{"bare fragment", "#" + block, "notion-" + block},
		{"no fragment", "/" + page, ""},
		{"non-block fragment", "/" + page + "#section-intro", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		if got := notionLinkFragmentAnchor(tc.raw); got != tc.want {
			t.Errorf("%s: notionLinkFragmentAnchor(%q) = %q, want %q", tc.name, tc.raw, got, tc.want)
		}
	}
}

func TestDirectSubpageLinksHandlesContentCycle(t *testing.T) {
	// A block-content cycle must not drive the traversal into infinite
	// recursion (test completion proves the guard works).
	rm := marshalRecordMap(t, map[string]any{
		"rootpage": map[string]any{"id": "rootpage", "type": "page", "content": []any{"cyc1"}},
		"cyc1":     map[string]any{"id": "cyc1", "type": "text", "content": []any{"cyc2"}},
		"cyc2":     map[string]any{"id": "cyc2", "type": "text", "content": []any{"cyc1"}},
	})
	links, err := DirectSubpageLinks(rm, "rootpage")
	if err != nil {
		t.Fatalf("DirectSubpageLinks error: %v", err)
	}
	if len(links) != 0 {
		t.Fatalf("expected no subpage links, got %v", links)
	}
}

func TestRenderTableOfContentsHandlesContentCycle(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	rm := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{"id": rootID, "type": "page", "content": []any{"toc", "h1", "box"}},
		"toc":  map[string]any{"id": "toc", "type": "table_of_contents"},
		"h1":   map[string]any{"id": "h1", "type": "header", "properties": map[string]any{"title": [][]any{{"Heading One"}}}},
		"box":  map[string]any{"id": "box", "type": "column", "content": []any{"box2"}},
		"box2": map[string]any{"id": "box2", "type": "column", "content": []any{"box"}},
	})
	html, err := RenderPage(RenderInput{RecordMap: rm, PageID: rootID})
	if err != nil {
		t.Fatalf("RenderPage error: %v", err)
	}
	if !strings.Contains(html, "notion-toc") || !strings.Contains(html, "Heading One") {
		t.Fatalf("table of contents should render the heading despite the content cycle: %s", html)
	}
}
