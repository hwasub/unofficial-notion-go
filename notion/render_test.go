package notion

import (
	"errors"
	"strings"
	"testing"
)

func TestRenderPagePreservesAllowlistedPageFormatClasses(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID},
			"format": map[string]any{
				"page_full_width": true,
				"page_small_text": true,
				"page_font":       "serif",
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Formatted page"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-body notion-body--full-width notion-body--small-text notion-body--font-serif"><p>Formatted page</p></div>`)
}

func TestRenderPageDropsUnsafePageFontClass(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":   rootID,
			"type": "page",
			"format": map[string]any{
				"page_font": `serif" onclick="alert(1)`,
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-body"></div>`)
	if strings.Contains(html, "onclick") || strings.Contains(html, "notion-body--font") {
		t.Fatalf("unsafe page font leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersRootTitleAsH1(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		headerID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{headerID},
			"properties": map[string]any{
				"title": [][]any{{`My <Title>`}},
			},
		},
		headerID: map[string]any{
			"id":   headerID,
			"type": "header",
			"properties": map[string]any{
				"title": [][]any{{"Section"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<h1 class="notion-page-title">My &lt;Title&gt;</h1>`)
	// The title is the body's single h1; content headings start at h2 and follow it.
	title := strings.Index(html, `<h1 class="notion-page-title">`)
	heading := strings.Index(html, "<h2")
	if title < 0 || heading < 0 || title > heading {
		t.Fatalf("expected page title h1 before the content h2: %s", html)
	}
}

func TestRenderPageEnforcesInputLimits(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		childID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{childID},
		},
		childID: map[string]any{
			"id":         childID,
			"type":       "text",
			"properties": map[string]any{"title": [][]any{{"hi"}}},
		},
	})

	if _, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID, MaxRecordMapBytes: 1}); !errors.Is(err, ErrRecordMapTooLarge) {
		t.Fatalf("MaxRecordMapBytes: err = %v, want ErrRecordMapTooLarge", err)
	}
	if _, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID, MaxBlocks: 1}); !errors.Is(err, ErrTooManyBlocks) {
		t.Fatalf("MaxBlocks: err = %v, want ErrTooManyBlocks", err)
	}
	if _, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID, MaxOutputBytes: 1}); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("MaxOutputBytes: err = %v, want ErrOutputTooLarge", err)
	}
	// Negative values disable each limit; the page renders normally.
	if _, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID, MaxRecordMapBytes: -1, MaxBlocks: -1, MaxOutputBytes: -1}); err != nil {
		t.Fatalf("disabled limits should render: %v", err)
	}
}

func TestRenderPageUntitledRootEmitsNoTitleH1(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":   rootID,
			"type": "page",
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-body"></div>`)
	if strings.Contains(html, "notion-page-title") {
		t.Fatalf("untitled page should not emit a title h1: %s", html)
	}
}

func TestRenderPageRendersBreadcrumbWithInternalPageLinks(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		childID      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		breadcrumbID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":   rootID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{`Root "Page"`}},
			},
		},
		childID: map[string]any{
			"id":        childID,
			"type":      "page",
			"parent_id": rootID,
			"content":   []string{breadcrumbID},
			"properties": map[string]any{
				"title": [][]any{{`Child <Page>`}},
			},
		},
		breadcrumbID: map[string]any{
			"id":        breadcrumbID,
			"type":      "breadcrumb",
			"parent_id": childID,
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       childID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{rootID: "", childID: childID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<nav class="notion-breadcrumb" aria-label="Breadcrumb"><ol><li><a href="/doc">Root &#34;Page&#34;</a></li><li aria-current="page"><span>Child &lt;Page&gt;</span></li></ol></nav>`)
	if strings.Contains(html, `<Page>`) || strings.Contains(html, `href="/doc/aaaaaaaa`) {
		t.Fatalf("breadcrumb rendered unsafe text or non-canonical root link: %s", html)
	}
}

func TestRenderPageRewritesSubpageAndAssetLinksSafely(t *testing.T) {
	const (
		rootID     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		subpageID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		imageID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		fileID     = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		badImageID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		bookmarkID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		textID     = "11111111-1111-1111-1111-111111111111"
		videoID    = "22222222-2222-2222-2222-222222222222"
		embedID    = "33333333-3333-3333-3333-333333333333"
		pdfID      = "44444444-4444-4444-4444-444444444444"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{subpageID, imageID, fileID, badImageID, bookmarkID, textID, videoID, embedID, pdfID},
		},
		subpageID: map[string]any{
			"id":   subpageID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{"Child <Page>"}},
			},
			"format": map[string]any{
				"page_icon": "📄",
			},
		},
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"properties": map[string]any{
				"caption": [][]any{{"hero <shot>"}},
			},
		},
		fileID: map[string]any{
			"id":   fileID,
			"type": "file",
			"properties": map[string]any{
				"title": [][]any{{"Spec PDF"}},
			},
		},
		badImageID: map[string]any{
			"id":   badImageID,
			"type": "image",
		},
		bookmarkID: map[string]any{
			"id":   bookmarkID,
			"type": "bookmark",
			"properties": map[string]any{
				"title":  [][]any{{"Bad Bookmark"}},
				"source": [][]any{{"javascript:alert(1)"}},
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Internal page", [][]any{{"a", "/e9f86f7a08604f51a4a8b7df25ad8ae3?pvs=25"}}}},
			},
		},
		videoID: map[string]any{
			"id":   videoID,
			"type": "video",
			"properties": map[string]any{
				"title":   [][]any{{"Watch"}},
				"source":  [][]any{{"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}},
				"caption": [][]any{{`Video caption <safe>`, [][]any{{"b"}}}},
			},
		},
		embedID: map[string]any{
			"id":   embedID,
			"type": "embed",
			"format": map[string]any{
				"display_source": "https://example.com/embed",
			},
		},
		pdfID: map[string]any{
			"id":   pdfID,
			"type": "pdf",
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       strings.ReplaceAll(rootID, "-", ""),
		ResourceSlug: "my doc/unsafe",
		PagePaths: map[string]string{
			subpageID: `x" onmouseover="alert(1)/한글`,
		},
		AssetURLs: map[string]string{
			imageID:    "/notion-assets/asset%201/hero.png",
			fileID:     "https://cdn.example.com/spec.pdf",
			badImageID: "javascript:alert(1)",
			pdfID:      "/notion-assets/spec/Spec%20Sheet.pdf",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, html, `href="/my%20doc%2Funsafe/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"`)
	assertContains(t, html, `<span class="notion-page-icon" aria-hidden="true">📄</span>Child &lt;Page&gt;`)
	assertContains(t, html, `Child &lt;Page&gt;`)
	assertContains(t, html, `<a class="notion-media__link" href="/notion-assets/asset%201/hero.png" data-collection-lightbox data-full-src="/notion-assets/asset%201/hero.png">`)
	assertContains(t, html, `src="/notion-assets/asset%201/hero.png"`)
	assertContains(t, html, `hero &lt;shot&gt;`)
	assertContains(t, html, `href="https://cdn.example.com/spec.pdf" rel="noopener noreferrer"><span class="notion-file__icon" aria-hidden="true"></span><span class="notion-file__name">Spec PDF</span></a>`)
	assertContains(t, html, `href="https://www.notion.so/e9f86f7a08604f51a4a8b7df25ad8ae3?pvs=25"`)
	assertContains(t, html, `class="notion-embed notion-embed--video notion-embed--youtube"`)
	assertContains(t, html, `src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"`)
	assertContains(t, html, `sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation"`)
	assertContains(t, html, `<figcaption><strong>Video caption &lt;safe&gt;</strong><br><a href="https://www.youtube.com/watch?v=dQw4w9WgXcQ" rel="noopener noreferrer">Watch</a></figcaption>`)
	assertContains(t, html, `href="https://www.youtube.com/watch?v=dQw4w9WgXcQ"`)
	assertContains(t, html, `class="notion-bookmark notion-bookmark--embed"`)
	assertContains(t, html, `href="https://example.com/embed"`)
	assertContains(t, html, `<span class="notion-bookmark__body"><strong>https://example.com/embed</strong><span class="notion-bookmark__host">example.com</span></span>`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--pdf"><iframe title="Spec Sheet.pdf" src="/notion-assets/spec/Spec%20Sheet.pdf"`)
	assertContains(t, html, `sandbox="allow-same-origin"`)
	assertContains(t, html, `Spec Sheet.pdf</a></figcaption></figure>`)
	if strings.Contains(html, "javascript:") {
		t.Fatalf("rendered HTML contains an unsafe javascript URL: %s", html)
	}
	if strings.Contains(html, `<video`) {
		t.Fatalf("external video URL rendered as a native video element: %s", html)
	}
	if strings.Contains(html, "Bad Bookmark") {
		t.Fatalf("unsafe bookmark was rendered: %s", html)
	}
	if strings.Contains(html, `" onmouseover=`) || strings.Contains(html, "/한글") {
		t.Fatalf("subpage href used unsafe title/path data: %s", html)
	}
	if strings.Count(html, `class="notion-page-icon"`) != 1 {
		t.Fatalf("subpage icon rendered an unexpected number of times: %s", html)
	}
}
func TestRenderPageRendersSanitizedSnapshotWarnings(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Visible body"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		Warnings: []RenderWarning{
			{Kind: RenderWarningFetchErrors, Count: 2},
			{Kind: RenderWarningFetchErrors, Count: 1},
			{Kind: RenderWarningAssetDownloadErrors, Count: 1},
			{Kind: RenderWarningBlocksTruncated, Count: 1},
			{Kind: "raw message <script>", Count: 1},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-render-warning" role="note">`)
	assertContains(t, html, `<strong>Some Notion content could not be loaded.</strong>`)
	assertContains(t, html, `<li>3 fetch warnings</li>`)
	assertContains(t, html, `<li>1 asset download warnings</li>`)
	assertContains(t, html, `<li>Block limit reached; some page content may be missing.</li>`)
	assertContains(t, html, `<p>Visible body</p>`)
	if strings.Contains(html, "raw message") || strings.Contains(html, "<script>") {
		t.Fatalf("render warning leaked raw warning kind: %s", html)
	}
}

func TestRenderPageResolvesRichTextLinksInCaptionsAndTables(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		targetID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		imageID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		codeID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		embedID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		tableID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		rowID    = "11111111-1111-1111-1111-111111111111"
	)
	targetCompact := strings.ReplaceAll(targetID, "-", "")
	pageLink := "/" + targetCompact + "?pvs=25"
	staleAssetLink := "https://file.notion.so/workspace/leak.png?signature=secret"
	linkText := func(label string) [][]any {
		return [][]any{
			{label, [][]any{{"a", pageLink}}},
			{" stale", [][]any{{"a", staleAssetLink}}},
		}
	}
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{imageID, codeID, embedID, tableID},
		},
		targetID: map[string]any{
			"id":   targetID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{"Cached target"}},
			},
		},
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"properties": map[string]any{
				"caption": linkText("Image caption"),
			},
		},
		codeID: map[string]any{
			"id":   codeID,
			"type": "code",
			"properties": map[string]any{
				"title":    [][]any{{"console.log('ok')"}},
				"language": [][]any{{"JavaScript"}},
				"caption":  linkText("Code caption"),
			},
		},
		embedID: map[string]any{
			"id":   embedID,
			"type": "embed",
			"properties": map[string]any{
				"source":  [][]any{{"https://www.youtube.com/watch?v=dQw4w9WgXcQ"}},
				"caption": linkText("Embed caption"),
			},
		},
		tableID: map[string]any{
			"id":      tableID,
			"type":    "table",
			"content": []string{rowID},
		},
		rowID: map[string]any{
			"id":   rowID,
			"type": "table_row",
			"properties": map[string]any{
				"cell": linkText("Table cell"),
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{targetID: targetID},
		AssetURLs:    map[string]string{imageID: "/notion-assets/image/hero.png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	internalHref := `href="/doc/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"`
	if got := strings.Count(html, internalHref); got < 4 {
		t.Fatalf("resolved internal page href count = %d, want at least 4 in captions/table: %s", got, html)
	}
	assertContains(t, html, `<figcaption><a href="/doc/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" rel="noopener noreferrer">Image caption</a> stale</figcaption>`)
	assertContains(t, html, `<div class="notion-code__caption"><a href="/doc/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" rel="noopener noreferrer">Code caption</a> stale</div>`)
	assertContains(t, html, `<a href="/doc/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" rel="noopener noreferrer">Embed caption</a> stale<br>`)
	assertContains(t, html, `<td><a href="/doc/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" rel="noopener noreferrer">Table cell</a> stale</td>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "signature=secret") {
		t.Fatalf("Notion-hosted signed rich-text link leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageResolvesInPageHeadingAnchorLink(t *testing.T) {
	const (
		rootID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		headingID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		paraID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	rootCompact := strings.ReplaceAll(rootID, "-", "")
	headingCompact := strings.ReplaceAll(headingID, "-", "")
	// In-page heading link as Notion stores it: "/<pageid>#<blockid>".
	pageLink := "/" + rootCompact + "#" + headingCompact
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{headingID, paraID},
		},
		headingID: map[string]any{
			"id":   headingID,
			"type": "header",
			"properties": map[string]any{
				"title": [][]any{{"Getting started"}},
			},
		},
		paraID: map[string]any{
			"id":   paraID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Jump to start", [][]any{{"a", pageLink}}}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{rootID: ""},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The heading carries the anchor id and the in-page link must point at it,
	// fragment included — otherwise clicking the link does nothing.
	assertContains(t, html, `<h2 id="notion-`+headingCompact+`">Getting started</h2>`)
	assertContains(t, html, `<a href="/doc#notion-`+headingCompact+`" rel="noopener noreferrer">Jump to start</a>`)
	if strings.Contains(html, `<a href="/doc" rel="noopener noreferrer">Jump to start</a>`) {
		t.Fatalf("in-page heading link dropped its #fragment: %s", html)
	}
}

func TestRenderPageUsesLocalPageIconAsset(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":   rootID,
			"type": "page",
			"format": map[string]any{
				"page_icon": "https://prod-files-secure.s3.us-west-2.amazonaws.com/workspace/icon.png?X-Amz-Signature=secret",
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			rootID + ":icon": "/notion-assets/icon/local.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-page-icon-hero" aria-hidden="true"><img src="/notion-assets/icon/local.png" alt=""></div>`)
	if strings.Contains(html, "prod-files-secure") || strings.Contains(html, "X-Amz-Signature") {
		t.Fatalf("page icon leaked the original Notion asset URL: %s", html)
	}
}

func TestRenderPageSuppressesUnstoredNotionPageIconURL(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":   rootID,
			"type": "page",
			"format": map[string]any{
				"page_icon": "https://s3-us-west-2.amazonaws.com/secure.notion-static.com/workspace/icon.png?X-Amz-Signature=secret",
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "notion-page-icon-hero") || strings.Contains(html, "secure.notion-static.com") || strings.Contains(html, "X-Amz-Signature") {
		t.Fatalf("unstored Notion page icon URL leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageSubpageHrefUsesPageIDForUnsafeTitles(t *testing.T) {
	const (
		rootID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		subpageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{subpageID},
		},
		subpageID: map[string]any{
			"id":   subpageID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{`x" onmouseover="alert(1)/한글`}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: `doc " unsafe/slug`,
		PagePaths: map[string]string{
			subpageID: `title/path" onmouseover="alert(1)`,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	assertContains(t, html, `href="/doc%20%22%20unsafe%2Fslug/bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"`)
	assertContains(t, html, `x&#34; onmouseover=&#34;alert(1)/한글`)
	if strings.Contains(html, `href="/doc%20%22%20unsafe%2Fslug/title`) {
		t.Fatalf("subpage href used title-derived path: %s", html)
	}
	if strings.Contains(html, `" onmouseover=`) {
		t.Fatalf("subpage href broke out of the href attribute: %s", html)
	}
}

func TestRenderPageSkipsInvalidSubpageIDs(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{"not-a-page-id"},
		},
		"not-a-page-id": map[string]any{
			"id":   "not-a-page-id",
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{`Unsafe Page`}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths: map[string]string{
			"not-a-page-id": "not-a-page-id",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, `<a href=`) {
		t.Fatalf("invalid subpage id was linked: %s", html)
	}
}

func TestRenderPageDoesNotRenderLinkedSubpageChildren(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		subpageID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		grandchildID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{subpageID},
		},
		subpageID: map[string]any{
			"id":      subpageID,
			"type":    "page",
			"content": []string{grandchildID},
			"properties": map[string]any{
				"title": [][]any{{"Linked child"}},
			},
		},
		grandchildID: map[string]any{
			"id":   grandchildID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Nested content should not render"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{subpageID: subpageID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, "Linked child")
	if strings.Contains(html, "Nested content should not render") {
		t.Fatalf("linked subpage children were rendered recursively: %s", html)
	}
}

func TestRenderPageRendersAliasPageLinks(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		aliasID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{aliasID},
		},
		aliasID: map[string]any{
			"id":   aliasID,
			"type": "alias",
			"format": map[string]any{
				"alias_pointer": map[string]any{"id": targetID, "table": "block"},
			},
		},
		targetID: map[string]any{
			"id":   targetID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{"Linked target"}},
			},
			"format": map[string]any{"page_icon": "📌"},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{targetID: targetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<p class="notion-subpage"><a href="/doc/cccccccc-cccc-cccc-cccc-cccccccccccc"><span class="notion-page-icon" aria-hidden="true">📌</span>Linked target</a></p>`)
}

func TestRenderPageLinksPageMentionsWithoutTargetBlock(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"See "}, {"Target page", [][]any{{"p", targetID}}}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{targetID: targetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `See <a class="notion-mention notion-mention--page" href="/doc/cccccccc-cccc-cccc-cccc-cccccccccccc">Target page</a>`)
}

func TestRenderPageRewritesCachedNotionTextLinks(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{
					{"Cached page", [][]any{{"a", "https://www.notion.so/workspace/Cached-Page-cccccccccccccccccccccccccccccccc?pvs=4"}}},
					{" and external page", [][]any{{"a", "https://www.notion.so/dddddddddddddddddddddddddddddddd"}}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{targetID: targetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<a href="/doc/cccccccc-cccc-cccc-cccc-cccccccccccc" rel="noopener noreferrer">Cached page</a>`)
	assertContains(t, html, `<a href="https://www.notion.so/dddddddddddddddddddddddddddddddd" rel="noopener noreferrer"> and external page</a>`)
	if strings.Contains(html, `Cached-Page-cccccccccccccccccccccccccccccccc`) {
		t.Fatalf("cached Notion text link was not rewritten internally: %s", html)
	}
}

func TestDirectSubpageLinksCollectsOnlyDirectLinkedPages(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		toggleID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		subpageID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		grandchildID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{toggleID},
		},
		toggleID: map[string]any{
			"id":      toggleID,
			"type":    "toggle",
			"content": []string{subpageID},
		},
		subpageID: map[string]any{
			"id":      subpageID,
			"type":    "page",
			"content": []string{grandchildID},
			"properties": map[string]any{
				"title": [][]any{{"Direct child"}},
			},
		},
		grandchildID: map[string]any{
			"id":   grandchildID,
			"type": "page",
			"properties": map[string]any{
				"title": [][]any{{"Grandchild"}},
			},
		},
	})

	links, err := DirectSubpageLinks(recordMap, rootID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 {
		t.Fatalf("links = %+v, want exactly one direct child", links)
	}
	if links[0].PageID != subpageID || links[0].Title != "Direct child" {
		t.Fatalf("link = %+v, want direct child %q", links[0], subpageID)
	}
}

func TestDirectSubpageLinksCollectsAliasPointers(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		aliasID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{aliasID},
		},
		aliasID: map[string]any{
			"id":   aliasID,
			"type": "alias",
			"properties": map[string]any{
				"title": [][]any{{"Alias title"}},
			},
			"format": map[string]any{
				"alias_pointer": map[string]any{"id": strings.ReplaceAll(targetID, "-", "")},
			},
		},
	})

	links, err := DirectSubpageLinks(recordMap, rootID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].PageID != targetID || links[0].Title != "Alias title" {
		t.Fatalf("DirectSubpageLinks alias links = %+v, want target alias title", links)
	}
}

func TestDirectSubpageLinksCollectsRichTextNotionPageLinks(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID       = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		grandchildID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{
					{"Open linked child", [][]any{{"a", "https://www.notion.so/Linked-Child-cccccccccccccccccccccccccccccccc"}}},
					{" and mention", [][]any{{"p", strings.ReplaceAll(targetID, "-", "")}}},
					{" and skip external", [][]any{{"a", "https://example.com/page"}}},
					{" and skip grandchild", [][]any{{"lm", "https://www.notion.so/Grandchild-dddddddddddddddddddddddddddddddd"}}},
				},
			},
		},
		targetID: map[string]any{
			"id":        targetID,
			"type":      "page",
			"parent_id": rootID,
			"properties": map[string]any{
				"title": [][]any{{"Linked child"}},
			},
		},
		grandchildID: map[string]any{
			"id":        grandchildID,
			"type":      "page",
			"parent_id": targetID,
			"properties": map[string]any{
				"title": [][]any{{"Grandchild"}},
			},
		},
	})

	links, err := DirectSubpageLinks(recordMap, rootID)
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].PageID != targetID || links[0].Title != "Linked child" {
		t.Fatalf("DirectSubpageLinks rich text links = %+v, want only root child %q", links, targetID)
	}
}

func TestRenderPageRendersButtonBlockAsStaticSnapshot(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		buttonID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{buttonID},
		},
		buttonID: map[string]any{
			"id":   buttonID,
			"type": "button",
			"properties": map[string]any{
				"title": [][]any{{`Run "sync"`, [][]any{{"b"}}}},
			},
			"format": map[string]any{
				"block_color":   "blue_background",
				"automation_id": `auto" onclick="alert(1)`,
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-button-block"><button type="button" class="notion-button notion-color--blue-background notion-button--inert" disabled aria-disabled="true"><span class="notion-button__label"><strong>Run &#34;sync&#34;</strong></span></button></div>`)
	if strings.Contains(html, "automation_id") || strings.Contains(html, "onclick") {
		t.Fatalf("button automation data leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersSafeOpenURLButtonAction(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		buttonID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		automationID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		actionID     = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{buttonID},
			},
			buttonID: map[string]any{
				"id":   buttonID,
				"type": "button",
				"format": map[string]any{
					"block_color":   "blue_background",
					"automation_id": automationID,
				},
			},
		},
		"automation": map[string]any{
			automationID: map[string]any{
				"id":         automationID,
				"action_ids": []string{actionID},
				"properties": map[string]any{
					"name": `Open "docs"`,
				},
			},
		},
		"automation_action": map[string]any{
			actionID: map[string]any{
				"id":   actionID,
				"type": "open_page",
				"config": map[string]any{
					"target": map[string]any{
						"type": "url",
						"url":  "https://example.com/docs",
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-button-block"><a class="notion-button notion-color--blue-background notion-button--link" href="https://example.com/docs" rel="noopener noreferrer"><span class="notion-button__label">Open &#34;docs&#34;</span></a></div>`)
	if strings.Contains(html, "automation_id") || strings.Contains(html, actionID) {
		t.Fatalf("button automation internals leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersSafeOpenPageButtonAction(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		buttonID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID     = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		automationID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		actionID     = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{buttonID},
			},
			buttonID: map[string]any{
				"id":   buttonID,
				"type": "button",
				"format": map[string]any{
					"automation_id": automationID,
				},
			},
		},
		"automation": map[string]any{
			automationID: map[string]any{
				"id":         automationID,
				"action_ids": []string{actionID},
				"properties": map[string]any{"name": "Open cached page"},
			},
		},
		"automation_action": map[string]any{
			actionID: map[string]any{
				"id":   actionID,
				"type": "open_page",
				"config": map[string]any{
					"target": map[string]any{
						"type":   "page",
						"pageId": targetID,
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "doc",
		PagePaths:    map[string]string{targetID: targetID},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<a class="notion-button notion-button--link" href="/doc/eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"><span class="notion-button__label">Open cached page</span></a>`)
}

func TestRenderPageDoesNotRenderUnsafeButtonActions(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		buttonID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		automationID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		actionID     = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	for name, action := range map[string]map[string]any{
		"javascript url": {
			"id":   actionID,
			"type": "open_page",
			"config": map[string]any{
				"target": map[string]any{
					"type": "url",
					"url":  "javascript:alert(1)",
				},
			},
		},
		"webhook": {
			"id":   actionID,
			"type": "send_webhook",
			"config": map[string]any{
				"url": "https://hooks.example.com/secret",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			recordMap := marshalRecordMapObject(t, map[string]any{
				"block": map[string]any{
					rootID: map[string]any{
						"id":      rootID,
						"type":    "page",
						"content": []string{buttonID},
					},
					buttonID: map[string]any{
						"id":   buttonID,
						"type": "button",
						"format": map[string]any{
							"automation_id": automationID,
						},
					},
				},
				"automation": map[string]any{
					automationID: map[string]any{
						"id":         automationID,
						"action_ids": []string{actionID},
						"properties": map[string]any{"name": "Unsafe action"},
					},
				},
				"automation_action": map[string]any{actionID: action},
			})

			html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
			if err != nil {
				t.Fatal(err)
			}
			assertContains(t, html, `<button type="button" class="notion-button notion-button--inert" disabled aria-disabled="true"><span class="notion-button__label">Unsafe action</span></button>`)
			if strings.Contains(html, "javascript:") || strings.Contains(html, "hooks.example.com") {
				t.Fatalf("unsafe action leaked into rendered HTML: %s", html)
			}
		})
	}
}

func TestRenderPageRendersTabBlocks(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tabID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		firstID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		secondID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		bodyID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		hiddenID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{tabID},
		},
		tabID: map[string]any{
			"id":      tabID,
			"type":    "tab",
			"content": []string{firstID, secondID},
		},
		firstID: map[string]any{
			"id":      firstID,
			"type":    "text",
			"content": []string{bodyID},
			"properties": map[string]any{
				"title": [][]any{{"Overview"}},
			},
		},
		secondID: map[string]any{
			"id":      secondID,
			"type":    "text",
			"content": []string{hiddenID},
			"properties": map[string]any{
				"title": [][]any{{`Unsafe "><script>alert(1)</script>`}},
			},
		},
		bodyID: map[string]any{
			"id":   bodyID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Visible panel"}},
			},
		},
		hiddenID: map[string]any{
			"id":   hiddenID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Second panel"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-tab-block" data-notion-tabs>`)
	assertContains(t, html, `role="tablist" aria-label="Tabs"`)
	assertContains(t, html, `data-notion-tab-target="notion-tab-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-panel-0"><span class="notion-tab-button__label">Overview</span></button>`)
	assertContains(t, html, `<span class="notion-tab-button__label">Unsafe &#34;&gt;&lt;script&gt;alert(1)&lt;/script&gt;</span></button>`)
	assertContains(t, html, `<section class="notion-tab-panel" role="tabpanel" tabindex="0" id="notion-tab-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-panel-1"`)
	assertContains(t, html, `data-notion-tab-panel hidden><p>Second panel</p></section>`)
	if strings.Contains(html, `<script>`) || strings.Contains(html, `"><script`) {
		t.Fatalf("tab label rendered unsafe markup: %s", html)
	}
}

func TestRenderPageRendersTransclusionReferenceContent(t *testing.T) {
	const (
		rootID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		refID       = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		containerID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		childID     = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{refID},
		},
		refID: map[string]any{
			"id":   refID,
			"type": "transclusion_reference",
			"format": map[string]any{
				"transclusion_reference_pointer": map[string]any{"id": containerID, "table": "block"},
			},
		},
		containerID: map[string]any{
			"id":      containerID,
			"type":    "transclusion_container",
			"content": []string{childID},
		},
		childID: map[string]any{
			"id":   childID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Copied content"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-synced-block"><p>Copied content</p></div>`)
	if strings.Contains(html, `<div class="notion-synced-block"></div>`) {
		t.Fatalf("transclusion reference rendered empty synced block: %s", html)
	}
}

func TestRenderPageRendersDateMentionsAndTableOfContents(t *testing.T) {
	const (
		rootID           = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tocID            = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		headerID         = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		textID           = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		externalObjectID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		genericObjectID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{tocID, headerID, textID},
		},
		tocID: map[string]any{
			"id":   tocID,
			"type": "table_of_contents",
		},
		headerID: map[string]any{
			"id":   headerID,
			"type": "header",
			"properties": map[string]any{
				"title": [][]any{{"Schedule"}},
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{
					{"‣", [][]any{{"d", map[string]any{
						"type":       "date",
						"start_date": "2026-06-01",
					}}}},
					{" highlight", [][]any{{"h", "blue_background"}}},
					{" ‣", [][]any{{"eoi", externalObjectID}}},
					{" ‣", [][]any{{"eoi", genericObjectID}}},
					{" ‣", [][]any{{"‣", []any{"https://example.com/report?x=1&unsafe=<tag>", "Example report"}}}},
					{" Docs", [][]any{{"lm", "https://docs.example.com/guide"}}},
					{" Bad", [][]any{{"lm", "javascript:alert(1)"}}},
				},
			},
		},
		externalObjectID: map[string]any{
			"id":   externalObjectID,
			"type": "external_object_instance",
			"format": map[string]any{
				"uri": "https://github.com/ExampleOrg/render-kit",
				"attributes": []map[string]any{
					{"id": "title", "name": "Name", "values": []any{"render-kit"}},
				},
			},
		},
		genericObjectID: map[string]any{
			"id":   genericObjectID,
			"type": "external_object_instance",
			"format": map[string]any{
				"original_url": "https://linear.app/acme/issue/ABC-1/fix-renderer",
				"domain":       "linear.app",
				"attributes": []map[string]any{
					{"id": "title", "name": "Name", "values": []any{"Fix renderer"}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<nav class="notion-toc"><ol>`)
	assertContains(t, html, `href="#notion-cccccccccccccccccccccccccccccccc"`)
	assertContains(t, html, `<span class="notion-date-mention"><time datetime="2026-06-01">Jun 1, 2026</time></span>`)
	assertContains(t, html, `<mark class="notion-highlight notion-color--blue-background"> highlight</mark>`)
	assertContains(t, html, `<a class="notion-external-object-mention notion-external-object-mention--github" href="https://github.com/ExampleOrg/render-kit" rel="noopener noreferrer"><span class="notion-external-object-mention__mark" aria-hidden="true">GH</span><span class="notion-external-object-mention__title">render-kit</span></a>`)
	assertContains(t, html, `<a class="notion-external-object-mention notion-external-object-mention--linear-app" href="https://linear.app/acme/issue/ABC-1/fix-renderer" rel="noopener noreferrer"><span class="notion-external-object-mention__mark" aria-hidden="true">EX</span><span class="notion-external-object-mention__title">Fix renderer</span></a>`)
	assertContains(t, html, `<a class="notion-mention notion-mention--link" href="https://example.com/report?x=1&amp;unsafe=&lt;tag&gt;" rel="noopener noreferrer">Example report</a>`)
	assertContains(t, html, `<a class="notion-mention notion-mention--link" href="https://docs.example.com/guide" rel="noopener noreferrer">Docs</a>`)
	assertContains(t, html, `<span class="notion-mention">Bad</span>`)
	if strings.Contains(html, "javascript:") {
		t.Fatalf("unsafe rich text link mention leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersLinkMentionMetadataAndCustomEmojiSafely(t *testing.T) {
	const (
		rootID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		emojiID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		missingID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		iconURL     = "https://file.notion.so/workspace/link-icon.png"
		emojiURL    = "https://prod-files-secure.s3.us-west-2.amazonaws.com/workspace/star.png"
		missingURL  = "https://file.notion.so/workspace/missing.png"
		externalURL = "https://cdn.example.com/safe-icon.png"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{textID},
			},
			textID: map[string]any{
				"id":   textID,
				"type": "text",
				"properties": map[string]any{
					"title": [][]any{
						{"Docs", [][]any{{"lm", map[string]any{
							"href":          "https://docs.example.com/guide",
							"title":         "Guide <One>",
							"link_provider": "Docs",
							"icon_url":      iconURL,
							"thumbnail_url": "https://file.notion.so/workspace/thumb.png",
						}}}},
						{" External", [][]any{{"lm", map[string]any{
							"href":          "https://example.com/external",
							"title":         "External",
							"link_provider": "Example",
							"icon_url":      externalURL,
						}}}},
						{" ", [][]any{{"ce", emojiID}}},
						{" fallback", [][]any{{"ce", missingID}}},
					},
				},
			},
		},
		"custom_emojis": map[string]string{
			emojiID:   emojiURL,
			missingID: missingURL,
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			iconURL:                   "/notion-assets/link/icon.png",
			"custom_emoji:" + emojiID: "/notion-assets/emoji/star.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<span class="notion-link-mention"><a class="notion-link-mention__link" href="https://docs.example.com/guide" rel="noopener noreferrer"><img class="notion-link-mention__icon" src="/notion-assets/link/icon.png" loading="lazy" decoding="async" alt=""><span class="notion-link-mention__provider">Docs</span><span class="notion-link-mention__title">Guide &lt;One&gt;</span></a></span>`)
	assertContains(t, html, `<img class="notion-link-mention__icon" src="https://cdn.example.com/safe-icon.png" loading="lazy" decoding="async" alt="">`)
	assertContains(t, html, `<img class="notion-custom-emoji" src="/notion-assets/emoji/star.png" loading="lazy" decoding="async" alt="Custom emoji">`)
	assertContains(t, html, ` fallback`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "prod-files-secure") {
		t.Fatalf("direct Notion-hosted mention asset leaked into rendered HTML: %s", html)
	}
}
