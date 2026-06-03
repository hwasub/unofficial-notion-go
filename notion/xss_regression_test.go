package notion

import (
	"strings"
	"testing"
)

// This file is an XSS REGRESSION corpus. Every test drives the public
// RenderPage entrypoint and asserts that the rendered HTML never contains a
// dangerous substring (a live scheme in an href/src, a raw <script>, an
// attribute breakout, an injected class/handler) and DOES contain the
// expected escaped/sanitized form. The intent is to lock in the renderer's
// HTML-safety invariants so a future change cannot silently reintroduce XSS.
//
// If an assertion here fails it means a real regression: a dangerous value
// reached the output. Fix the renderer, do not relax the assertion.

const xssRootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

// dangerousSchemeSubstrings are scheme/payload fragments that must never appear
// anywhere in rendered HTML. They are checked verbatim (the renderer emits
// HTML-escaped output, and none of these fragments contain characters that
// would be altered by HTML escaping, so a raw match is a true positive).
//
// Note: bare event-handler tokens like "onerror=" are deliberately NOT in this
// list. When a payload such as `"><img src=x onerror=alert(1)>` is escaped, the
// literal text "onerror=" survives as inert page text between escaped angle
// brackets, which is safe. Event-handler breakout is instead asserted with the
// specific dangerous signature (handler immediately after a real quote/tag) in
// the tests that exercise attribute contexts.
var dangerousSchemeSubstrings = []string{
	"javascript:",
	"data:text/html",
	"vbscript:",
	"data:application/",
}

func assertNoDangerousSchemes(t *testing.T, html string) {
	t.Helper()
	for _, bad := range dangerousSchemeSubstrings {
		if strings.Contains(html, bad) {
			t.Fatalf("rendered HTML contains dangerous substring %q: %s", bad, html)
		}
	}
}

func renderXSSPage(t *testing.T, input RenderInput) string {
	t.Helper()
	html, err := RenderPage(input)
	if err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	return html
}

// TestXSSRichTextLinkAnnotationDropsUnsafeSchemes verifies that a malicious
// href on a rich-text link annotation ("a") is never emitted: the link is
// dropped (no <a> wrapper) and the visible text is preserved, escaped.
func TestXSSRichTextLinkAnnotationDropsUnsafeSchemes(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	cases := []struct {
		name string
		href string
	}{
		{"javascript", "javascript:alert(1)"},
		{"data_text_html", "data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg=="},
		{"vbscript", "vbscript:msgbox(1)"},
		{"leading_space", "  javascript:alert(1)"},
		{"leading_control", "\t\n javascript:alert(1)"},
		{"mixed_case", "JaVaScRiPt:alert(1)"},
		{"embedded_tab", "jav\tascript:alert(1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordMap := marshalRecordMap(t, map[string]any{
				xssRootID: map[string]any{
					"id":      xssRootID,
					"type":    "page",
					"content": []string{textID},
				},
				textID: map[string]any{
					"id":   textID,
					"type": "text",
					"properties": map[string]any{
						"title": [][]any{{"click me", [][]any{{"a", tc.href}}}},
					},
				},
			})

			html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
			assertNoDangerousSchemes(t, html)
			// The unsafe link must be dropped entirely; only the escaped text
			// remains, never an <a href="..."> pointing at the payload.
			if strings.Contains(html, `<a href=`) {
				t.Fatalf("unsafe rich-text link rendered an anchor: %s", html)
			}
			assertContains(t, html, "click me")
		})
	}
}

// TestXSSRichTextLinkAnnotationKeepsSafeHrefs is the positive control for the
// link-annotation path: a legitimate https link must still render as an anchor
// with the safe href intact, so the negative tests above are not vacuous.
func TestXSSRichTextLinkAnnotationKeepsSafeHrefs(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	recordMap := marshalRecordMap(t, map[string]any{
		xssRootID: map[string]any{
			"id":      xssRootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"safe link", [][]any{{"a", "https://example.com/ok"}}}},
			},
		},
	})

	html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
	assertContains(t, html, `<a href="https://example.com/ok" rel="noopener noreferrer">safe link</a>`)
}

// TestXSSImageAndMediaSourceDropsUnsafeSchemes verifies that image / file /
// embed-style blocks whose source is a dangerous or unexpected scheme never
// produce a src (or href) carrying that scheme.
func TestXSSImageAndMediaSourceDropsUnsafeSchemes(t *testing.T) {
	const (
		blockID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		sentID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	cases := []struct {
		name      string
		blockType string
		source    string
	}{
		{"image_javascript", "image", "javascript:alert(1)"},
		{"image_data_html", "image", "data:text/html;base64,PHN2Zz48L3N2Zz4="},
		{"image_vbscript", "image", "vbscript:msgbox(1)"},
		{"file_javascript", "file", "javascript:alert(1)"},
		{"file_data_html", "file", "data:text/html,<script>alert(1)</script>"},
		{"embed_javascript", "embed", "javascript:alert(1)"},
		{"video_javascript", "video", "javascript:alert(1)"},
		{"pdf_data_html", "pdf", "data:text/html,<script>alert(1)</script>"},
		{"bookmark_vbscript", "bookmark", "vbscript:msgbox(1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordMap := marshalRecordMap(t, map[string]any{
				xssRootID: map[string]any{
					"id":      xssRootID,
					"type":    "page",
					"content": []string{blockID, sentID},
				},
				blockID: map[string]any{
					"id":   blockID,
					"type": tc.blockType,
					"properties": map[string]any{
						"source": [][]any{{tc.source}},
						"title":  [][]any{{"media title"}},
					},
				},
				// Sentinel block proves rendering proceeded normally and the
				// page did not error out before/after the unsafe block.
				sentID: map[string]any{
					"id":   sentID,
					"type": "text",
					"properties": map[string]any{
						"title": [][]any{{"sentinel body"}},
					},
				},
			})

			html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
			assertNoDangerousSchemes(t, html)
			if strings.Contains(html, `src="javascript:`) || strings.Contains(html, `src="data:`) ||
				strings.Contains(html, `src="vbscript:`) {
				t.Fatalf("unsafe scheme leaked into a src attribute: %s", html)
			}
			if strings.Contains(html, `href="javascript:`) || strings.Contains(html, `href="data:`) ||
				strings.Contains(html, `href="vbscript:`) {
				t.Fatalf("unsafe scheme leaked into an href attribute: %s", html)
			}
			assertContains(t, html, "<p>sentinel body</p>")
		})
	}
}

// TestXSSImageRendersSafeExternalSource is the positive control for the image
// path: a legitimate https image source still renders, proving the negative
// image tests are not passing simply because images never render.
func TestXSSImageRendersSafeExternalSource(t *testing.T) {
	const imageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	recordMap := marshalRecordMap(t, map[string]any{
		xssRootID: map[string]any{
			"id":      xssRootID,
			"type":    "page",
			"content": []string{imageID},
		},
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"properties": map[string]any{
				"source": [][]any{{"https://images.example.com/photo.png"}},
			},
		},
	})

	html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
	assertContains(t, html, `src="https://images.example.com/photo.png"`)
}

// TestXSSTextContentEscapesHTMLMetacharacters verifies that HTML
// metacharacters in text content are emitted escaped and never as raw markup.
func TestXSSTextContentEscapesHTMLMetacharacters(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	cases := []struct {
		name    string
		payload string
		escaped string
	}{
		{
			name:    "script_tag",
			payload: "<script>alert(1)</script>",
			escaped: "&lt;script&gt;alert(1)&lt;/script&gt;",
		},
		{
			name:    "img_onerror_breakout",
			payload: `"><img src=x onerror=alert(1)>`,
			escaped: `&#34;&gt;&lt;img src=x onerror=alert(1)&gt;`,
		},
		{
			name:    "ampersand_and_quotes",
			payload: `Tom & "Jerry" <them>`,
			escaped: `Tom &amp; &#34;Jerry&#34; &lt;them&gt;`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordMap := marshalRecordMap(t, map[string]any{
				xssRootID: map[string]any{
					"id":      xssRootID,
					"type":    "page",
					"content": []string{textID},
				},
				textID: map[string]any{
					"id":   textID,
					"type": "text",
					"properties": map[string]any{
						"title": [][]any{{tc.payload}},
					},
				},
			})

			html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
			assertContains(t, html, tc.escaped)
			if strings.Contains(html, "<script>") || strings.Contains(html, "</script>") {
				t.Fatalf("raw <script> markup leaked into rendered HTML: %s", html)
			}
			if strings.Contains(html, "<img src=x") {
				t.Fatalf("raw injected <img> markup leaked into rendered HTML: %s", html)
			}
			assertNoDangerousSchemes(t, html)
		})
	}
}

// TestXSSAttributeBreakoutInTitlesAndCaptions verifies that values placed into
// attribute contexts (image alt, file name, toggle summary) cannot break out of
// the double-quoted attribute even when they contain a double quote and >.
func TestXSSAttributeBreakoutInTitlesAndCaptions(t *testing.T) {
	const (
		imageID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		fileID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		toggleID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	const breakout = `" onmouseover="alert(1)`
	const breakoutGT = `x"><script>alert(1)</script>`
	recordMap := marshalRecordMap(t, map[string]any{
		xssRootID: map[string]any{
			"id":      xssRootID,
			"type":    "page",
			"content": []string{imageID, fileID, toggleID},
		},
		// Image caption becomes the alt attribute (an attribute context).
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"properties": map[string]any{
				"source":  [][]any{{"https://images.example.com/photo.png"}},
				"caption": [][]any{{breakout}},
			},
		},
		// File title is used as the rendered link text / placeholder name.
		fileID: map[string]any{
			"id":   fileID,
			"type": "file",
			"properties": map[string]any{
				"title":  [][]any{{breakoutGT}},
				"source": [][]any{{"https://cdn.example.com/spec.pdf"}},
			},
		},
		// Toggle with no title falls back to a localized summary; here we give
		// it a malicious title to confirm it is escaped in the <summary>.
		toggleID: map[string]any{
			"id":   toggleID,
			"type": "toggle",
			"properties": map[string]any{
				"title": [][]any{{breakoutGT}},
			},
		},
	})

	html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
	assertNoDangerousSchemes(t, html)
	// The alt attribute must contain the escaped quote, never a literal one
	// that would terminate the attribute and start a new handler.
	if strings.Contains(html, `alt="" onmouseover=`) {
		t.Fatalf("attribute breakout via image alt: %s", html)
	}
	assertContains(t, html, `alt="&#34; onmouseover=&#34;alert(1)"`)
	// File name / toggle summary text must be escaped, no raw script tag.
	assertContains(t, html, `x&#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;`)
	if strings.Contains(html, "<script>") || strings.Contains(html, "</script>") {
		t.Fatalf("attribute breakout produced raw <script>: %s", html)
	}
}

// TestXSSBlockColorIsWhitelisted verifies that a malicious block_color token
// cannot break out of the class attribute or inject markup: only whitelisted
// notion-color-- classes are ever emitted.
func TestXSSBlockColorIsWhitelisted(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	cases := []struct {
		name  string
		color string
	}{
		{"quote_gt_script", `red"><script>`},
		{"handler_token", `x" onload="y`},
		{"close_tag", `default"></p><script>alert(1)</script>`},
		{"unknown_color", "chartreuse"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordMap := marshalRecordMap(t, map[string]any{
				xssRootID: map[string]any{
					"id":      xssRootID,
					"type":    "page",
					"content": []string{textID},
				},
				textID: map[string]any{
					"id":   textID,
					"type": "text",
					"properties": map[string]any{
						"title": [][]any{{"colored text"}},
					},
					"format": map[string]any{
						"block_color": tc.color,
					},
				},
			})

			html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
			assertNoDangerousSchemes(t, html)
			if strings.Contains(html, "<script>") || strings.Contains(html, "</script>") {
				t.Fatalf("malicious color token injected a script tag: %s", html)
			}
			// No attribute breakout: a raw quote must never appear inside the
			// class value, and the unsafe token must not survive.
			if strings.Contains(html, `class="red"`) || strings.Contains(html, `onload=`) {
				t.Fatalf("malicious color token broke out of the class attribute: %s", html)
			}
			assertContains(t, html, "colored text")
		})
	}
}

// TestXSSAnnotationColorIsWhitelisted verifies the rich-text color annotation
// ("h") only ever yields a whitelisted notion-color-- class, even when given a
// malicious color value.
func TestXSSAnnotationColorIsWhitelisted(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	cases := []struct {
		name  string
		color string
	}{
		{"span_breakout", `red"><script>`},
		{"handler_token", `x onload=y`},
		{"unknown_color", "neon"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recordMap := marshalRecordMap(t, map[string]any{
				xssRootID: map[string]any{
					"id":      xssRootID,
					"type":    "page",
					"content": []string{textID},
				},
				textID: map[string]any{
					"id":   textID,
					"type": "text",
					"properties": map[string]any{
						"title": [][]any{{"tinted", [][]any{{"h", tc.color}}}},
					},
				},
			})

			html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
			assertNoDangerousSchemes(t, html)
			if strings.Contains(html, "<script>") || strings.Contains(html, "</script>") {
				t.Fatalf("malicious annotation color injected a script tag: %s", html)
			}
			if strings.Contains(html, `notion-color--red"`) || strings.Contains(html, `onload=`) {
				t.Fatalf("malicious annotation color broke out of the class attribute: %s", html)
			}
			assertContains(t, html, "tinted")
		})
	}
}

// TestXSSAnnotationColorKeepsWhitelistedClass is the positive control for the
// color path: a legitimate color annotation still produces its sanitized class.
func TestXSSAnnotationColorKeepsWhitelistedClass(t *testing.T) {
	const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	recordMap := marshalRecordMap(t, map[string]any{
		xssRootID: map[string]any{
			"id":      xssRootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"tinted", [][]any{{"h", "blue"}}}},
			},
		},
	})

	html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
	assertContains(t, html, `<span class="notion-color--blue">tinted</span>`)
}

// TestXSSLocalizerOutputIsEscaped verifies that a Localizer returning HTML
// metacharacters cannot inject markup: RenderPage treats Localizer output as
// plain text and escapes it before embedding. This guards the Localizer
// escaping fix.
func TestXSSLocalizerOutputIsEscaped(t *testing.T) {
	const maliciousLocalized = `<b>&"x`
	const escapedLocalized = `&lt;b&gt;&amp;&#34;x`
	loc := func(key, fallback string) string {
		return maliciousLocalized
	}

	t.Run("render_warning_title", func(t *testing.T) {
		const textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		recordMap := marshalRecordMap(t, map[string]any{
			xssRootID: map[string]any{
				"id":      xssRootID,
				"type":    "page",
				"content": []string{textID},
			},
			textID: map[string]any{
				"id":   textID,
				"type": "text",
				"properties": map[string]any{
					"title": [][]any{{"body"}},
				},
			},
		})

		html := renderXSSPage(t, RenderInput{
			RecordMap: recordMap,
			PageID:    xssRootID,
			T:         loc,
			Warnings: []RenderWarning{
				{Kind: RenderWarningBlocksTruncated, Count: 1},
			},
		})
		assertContains(t, html, escapedLocalized)
		if strings.Contains(html, maliciousLocalized) {
			t.Fatalf("Localizer output was embedded without escaping: %s", html)
		}
	})

	t.Run("toggle_summary_fallback", func(t *testing.T) {
		const toggleID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		recordMap := marshalRecordMap(t, map[string]any{
			xssRootID: map[string]any{
				"id":      xssRootID,
				"type":    "page",
				"content": []string{toggleID},
			},
			// Empty title forces the localized "Details" fallback into the
			// <summary>, exercising firstText's escaping of the fallback.
			toggleID: map[string]any{
				"id":   toggleID,
				"type": "toggle",
			},
		})

		html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID, T: loc})
		assertContains(t, html, `<summary>`+escapedLocalized+`</summary>`)
		if strings.Contains(html, maliciousLocalized) {
			t.Fatalf("Localizer fallback output was embedded without escaping: %s", html)
		}
	})
}

// TestXSSBenignPageRendersExpectedSafeHTML is the sanity / non-vacuity control:
// a normal page renders the expected safe HTML, so the corpus is not passing
// merely because nothing renders.
func TestXSSBenignPageRendersExpectedSafeHTML(t *testing.T) {
	const (
		headingID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		textID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		bulletID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		linkID    = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		xssRootID: map[string]any{
			"id":      xssRootID,
			"type":    "page",
			"content": []string{headingID, textID, bulletID, linkID},
		},
		headingID: map[string]any{
			"id":   headingID,
			"type": "header",
			"properties": map[string]any{
				"title": [][]any{{"Hello"}},
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"A ", [][]any{{"b"}}}, {"bold", [][]any{{"b"}}}, {" word."}},
			},
		},
		bulletID: map[string]any{
			"id":   bulletID,
			"type": "bulleted_list",
			"properties": map[string]any{
				"title": [][]any{{"First item"}},
			},
		},
		linkID: map[string]any{
			"id":   linkID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"docs", [][]any{{"a", "https://example.com/docs"}}}},
			},
		},
	})

	html := renderXSSPage(t, RenderInput{RecordMap: recordMap, PageID: xssRootID})
	assertContains(t, html, `<div class="notion-body">`)
	assertContains(t, html, "Hello")
	assertContains(t, html, "<strong>bold</strong>")
	assertContains(t, html, "<li>First item</li>")
	assertContains(t, html, `<a href="https://example.com/docs" rel="noopener noreferrer">docs</a>`)
	assertNoDangerousSchemes(t, html)
	if strings.Contains(html, "<script>") {
		t.Fatalf("benign page unexpectedly contains a script tag: %s", html)
	}
}
