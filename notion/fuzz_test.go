package notion

import (
	"strings"
	"testing"
)

// FuzzRenderPage feeds arbitrary strings into the renderer through two sinks — a
// link href and visible text — and asserts the core HTML-safety invariants hold
// for any input: every emitted href/src has a safe scheme, the output never
// contains a raw <script tag, and RenderPage never panics.
//
// Run it with: go test -run '^$' -fuzz=FuzzRenderPage -fuzztime=60s ./notion
func FuzzRenderPage(f *testing.F) {
	seeds := []string{
		"javascript:alert(1)",
		"JaVaScRiPt:alert(1)",
		"data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==",
		"vbscript:msgbox(1)",
		`https:\\evil.example.com\x`,
		`/\evil.example.com/phish`,
		"\t\n javascript:alert(1)",
		`"><script>alert(1)</script>`,
		`"><img src=x onerror=alert(1)>`,
		"https://example.com/learn/javascript:tips",
		"https://example.com/ok",
		"plain text",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	const textID = "cccccccc-cccc-cccc-cccc-cccccccccccc"

	f.Fuzz(func(t *testing.T, s string) {
		// Sink 1: the fuzzed value as a link href (visible text is fixed).
		hrefMap := marshalRecordMap(t, map[string]any{
			xssRootID: map[string]any{"id": xssRootID, "type": "page", "content": []string{textID}},
			textID: map[string]any{
				"id":         textID,
				"type":       "text",
				"properties": map[string]any{"title": [][]any{{"link", [][]any{{"a", s}}}}},
			},
		})
		if html, err := RenderPage(RenderInput{RecordMap: hrefMap, PageID: xssRootID}); err == nil {
			assertNoDangerousHrefOrSrc(t, html)
			assertNoRawScript(t, html)
		}

		// Sink 2: the fuzzed value as visible text content.
		textMap := marshalRecordMap(t, map[string]any{
			xssRootID: map[string]any{"id": xssRootID, "type": "page", "content": []string{textID}},
			textID: map[string]any{
				"id":         textID,
				"type":       "text",
				"properties": map[string]any{"title": [][]any{{s}}},
			},
		})
		if html, err := RenderPage(RenderInput{RecordMap: textMap, PageID: xssRootID}); err == nil {
			assertNoRawScript(t, html)
		}
	})
}

// assertNoDangerousHrefOrSrc checks the scheme of every emitted href/src value
// rather than a raw substring, so a benign URL whose path happens to contain
// "javascript:" (e.g. https://x/javascript:tips) is not a false positive while a
// real javascript:/data:/vbscript: scheme is caught.
func assertNoDangerousHrefOrSrc(t *testing.T, html string) {
	t.Helper()
	for _, attr := range []string{`href="`, `src="`} {
		rest := html
		for {
			i := strings.Index(rest, attr)
			if i < 0 {
				break
			}
			rest = rest[i+len(attr):]
			j := strings.Index(rest, `"`)
			if j < 0 {
				break
			}
			if dangerousScheme(rest[:j]) {
				t.Fatalf("rendered %s%s\" has a dangerous scheme:\n%s", attr, rest[:j], html)
			}
			rest = rest[j+1:]
		}
	}
}

func dangerousScheme(url string) bool {
	u := strings.ToLower(strings.TrimSpace(url))
	for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
		if strings.HasPrefix(u, scheme) {
			return true
		}
	}
	return false
}

func assertNoRawScript(t *testing.T, html string) {
	t.Helper()
	if strings.Contains(strings.ToLower(html), "<script") {
		t.Fatalf("rendered HTML contains a raw <script tag:\n%s", html)
	}
}
