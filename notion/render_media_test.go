package notion

import (
	"strings"
	"testing"
)

func TestRenderPageRendersBookmarkMetadataSafely(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		bookmarkID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		signedIconID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{bookmarkID, signedIconID},
		},
		bookmarkID: map[string]any{
			"id":   bookmarkID,
			"type": "bookmark",
			"properties": map[string]any{
				"link":        [][]any{{"https://example.com/post"}},
				"title":       [][]any{{"Example <Post>"}},
				"description": [][]any{{`A "useful" summary <script>`}},
			},
			"format": map[string]any{
				"bookmark_icon":  "https://cdn.example.com/favicon.png",
				"bookmark_cover": "https://cdn.example.com/cover.jpg",
			},
		},
		signedIconID: map[string]any{
			"id":   signedIconID,
			"type": "bookmark",
			"properties": map[string]any{
				"link":  [][]any{{"https://example.org"}},
				"title": [][]any{{"No stale signed assets"}},
			},
			"format": map[string]any{
				"bookmark_icon":  "https://file.notion.so/workspace/favicon.png",
				"bookmark_cover": "javascript:alert(1)",
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-bookmark notion-bookmark--bookmark notion-bookmark--with-cover"><a href="https://example.com/post"`)
	assertContains(t, html, `<span class="notion-bookmark__title-row"><img class="notion-bookmark__icon" src="https://cdn.example.com/favicon.png" loading="lazy" decoding="async" alt=""><strong>Example &lt;Post&gt;</strong></span>`)
	assertContains(t, html, `<span class="notion-bookmark__description">A &#34;useful&#34; summary &lt;script&gt;</span>`)
	assertContains(t, html, `<span class="notion-bookmark__cover"><img src="https://cdn.example.com/cover.jpg" loading="lazy" decoding="async" alt=""></span>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "javascript:") {
		t.Fatalf("unsafe bookmark visual URL leaked into rendered HTML: %s", html)
	}
}
func TestRenderPageRendersExternalObjectGitHubCardsSafely(t *testing.T) {
	const (
		rootID           = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		externalObjectID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		signedIconID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		genericObjectID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{externalObjectID, signedIconID, genericObjectID},
		},
		externalObjectID: map[string]any{
			"id":   externalObjectID,
			"type": "external_object_instance",
			"format": map[string]any{
				"original_url": "https://github.com/ExampleOrg/render-kit",
				"domain":       "github.com",
				"attributes": []map[string]any{
					{"id": "title", "name": "Name", "values": []any{`Unsafe <Title>`}},
					{"id": "owner", "name": "Owner", "values": []any{"https://github.com/ExampleOrg"}},
					{"id": "updated_at", "name": "Updated At", "values": []any{map[string]any{
						"type":       "datetime",
						"start_date": "2026-06-01",
					}}},
					{
						"id":       "avatar_url",
						"name":     "Avatar URL",
						"type":     "embed",
						"mimeType": "image/*",
						"format":   map[string]any{"type": "icon"},
						"values":   []any{"https://avatars.githubusercontent.com/u/70270177?v=4"},
					},
				},
			},
		},
		signedIconID: map[string]any{
			"id":   signedIconID,
			"type": "external_object_instance",
			"format": map[string]any{
				"original_url": "https://github.com/NotionX",
				"attributes": []map[string]any{
					{"id": "title", "name": "Name", "values": []any{"No signed icon"}},
					{
						"id":       "avatar_url",
						"name":     "Avatar URL",
						"type":     "embed",
						"mimeType": "image/*",
						"format":   map[string]any{"type": "icon"},
						"values":   []any{"https://file.notion.so/workspace/avatar.png?signature=secret"},
					},
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
					{"id": "title", "name": "Name", "values": []any{`Fix <renderer>`}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-external-object notion-external-object--github"><a class="notion-external-object__link" href="https://github.com/ExampleOrg/render-kit" rel="noopener noreferrer">`)
	assertContains(t, html, `<span class="notion-external-object__icon" aria-hidden="true"><img src="https://avatars.githubusercontent.com/u/70270177?v=4" loading="lazy" decoding="async" alt=""></span>`)
	assertContains(t, html, `<span class="notion-external-object__eyebrow">GitHub</span><strong class="notion-external-object__title">Unsafe &lt;Title&gt;</strong>`)
	assertContains(t, html, `<span class="notion-external-object__meta">ExampleOrg · Jun 1, 2026</span>`)
	assertContains(t, html, `<strong class="notion-external-object__title">No signed icon</strong>`)
	assertContains(t, html, `<figure class="notion-external-object notion-external-object--linear-app"><a class="notion-external-object__link" href="https://linear.app/acme/issue/ABC-1/fix-renderer" rel="noopener noreferrer">`)
	assertContains(t, html, `<span class="notion-external-object__icon" aria-hidden="true"><span>EX</span></span><span class="notion-external-object__body"><span class="notion-external-object__eyebrow">linear.app</span><strong class="notion-external-object__title">Fix &lt;renderer&gt;</strong>`)
	if strings.Contains(html, `<Title>`) || strings.Contains(html, "file.notion.so") || strings.Contains(html, "signature=secret") {
		t.Fatalf("external object title was not escaped: %s", html)
	}
}

func TestRenderPageSuppressesUncachedNotionHostedFileMediaSources(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		fileID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		pdfID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		audioID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		videoID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{fileID, pdfID, audioID, videoID},
		},
		fileID: map[string]any{
			"id":   fileID,
			"type": "file",
			"properties": map[string]any{
				"title":  [][]any{{"Spec <unsafe>.pdf"}},
				"source": [][]any{{"https://file.notion.so/workspace/spec.pdf?signature=stale"}},
			},
		},
		pdfID: map[string]any{
			"id":   pdfID,
			"type": "pdf",
			"properties": map[string]any{
				"source": [][]any{{"https://prod-files-secure.s3.us-west-2.amazonaws.com/workspace/%3Cdoc%3E.pdf"}},
			},
		},
		audioID: map[string]any{
			"id":   audioID,
			"type": "audio",
			"properties": map[string]any{
				"title":  [][]any{{"Audio"}},
				"source": [][]any{{"https://secure.notion-static.com/audio.mp3"}},
			},
		},
		videoID: map[string]any{
			"id":   videoID,
			"type": "video",
			"properties": map[string]any{
				"title":  [][]any{{"Video"}},
				"source": [][]any{{"https://s3.us-west-2.amazonaws.com/video.mp4"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<p class="notion-file notion-file--unavailable"><span class="notion-file__placeholder"><span class="notion-file__icon" aria-hidden="true"></span><span class="notion-file__name">Spec &lt;unsafe&gt;.pdf</span></span></p>`)
	assertContains(t, html, `<p class="notion-file notion-file--unavailable"><span class="notion-file__placeholder"><span class="notion-file__icon" aria-hidden="true"></span><span class="notion-file__name">&lt;doc&gt;.pdf</span></span></p>`)
	assertContains(t, html, `<span class="notion-file__name">Audio</span>`)
	assertContains(t, html, `<span class="notion-file__name">Video</span>`)
	if strings.Contains(html, `href="https://file.notion.so`) ||
		strings.Contains(html, `href="https://prod-files-secure`) ||
		strings.Contains(html, `href="https://secure.notion-static.com`) ||
		strings.Contains(html, `href="https://s3.us-west-2.amazonaws.com`) ||
		strings.Contains(html, `<audio`) || strings.Contains(html, `<video`) {
		t.Fatalf("uncached Notion-hosted file/media source leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageUnsafeRendersNotionSignedMediaURLs(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		imageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		fileID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		videoID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	imageSignedURL := "https://file.notion.so/workspace/hero.png?signature=secret"
	fileSignedURL := "https://file.notion.so/workspace/spec.pdf?signature=secret"
	videoSourceURL := "https://file.notion.so/workspace/video.mp4?signature=secret"
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{imageID, fileID, videoID},
			},
			imageID: map[string]any{
				"id":   imageID,
				"type": "image",
				"properties": map[string]any{
					"source":  [][]any{{"attachment:hero.png"}},
					"caption": [][]any{{"Signed image"}},
				},
			},
			fileID: map[string]any{
				"id":   fileID,
				"type": "file",
				"properties": map[string]any{
					"title":  [][]any{{"Spec.pdf"}},
					"source": [][]any{{"attachment:spec.pdf"}},
				},
			},
			videoID: map[string]any{
				"id":   videoID,
				"type": "video",
				"properties": map[string]any{
					"source": [][]any{{videoSourceURL}},
				},
			},
		},
		"signed_urls": map[string]any{
			imageID: imageSignedURL,
			fileID:  fileSignedURL,
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:                    recordMap,
		PageID:                       rootID,
		UnsafeRenderNotionSignedURLs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<a class="notion-media__link" href="`+imageSignedURL+`" data-collection-lightbox data-full-src="`+imageSignedURL+`"><img src="`+imageSignedURL+`"`)
	assertContains(t, html, `<p class="notion-file"><a href="`+fileSignedURL+`" rel="noopener noreferrer">`)
	assertContains(t, html, `<video controls preload="metadata" src="`+videoSourceURL+`"`)
	if strings.Contains(html, "notion-file--unavailable") {
		t.Fatalf("unsafe signed URL option still rendered unavailable placeholders: %s", html)
	}
}

func TestRenderPageRendersImageFromSourceAssetAlias(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		imageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	sourceURL := "https://file.notion.so/workspace/source-image.png?signature=secret"
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{imageID},
		},
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"properties": map[string]any{
				"source":  [][]any{{sourceURL}},
				"caption": [][]any{{"Source alias image"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			"https://file.notion.so/workspace/source-image.png": "/notion-assets/image/source-image.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<a class="notion-media__link" href="/notion-assets/image/source-image.png" data-collection-lightbox data-full-src="/notion-assets/image/source-image.png"><img src="/notion-assets/image/source-image.png"`)
	assertContains(t, html, `<figcaption>Source alias image</figcaption>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "signature=secret") {
		t.Fatalf("rendered image leaked the original Notion signed URL: %s", html)
	}
}

func TestRenderPageRendersExternalImagesWithoutCaching(t *testing.T) {
	const (
		rootID        = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		externalID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		notionID      = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		httpID        = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		externalCover = "https://images.example.com/cover.png"
		externalImage = "https://images.example.com/photo.png"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{externalID, notionID, httpID},
			"format": map[string]any{
				"page_cover": externalCover,
			},
		},
		externalID: map[string]any{
			"id":   externalID,
			"type": "image",
			"properties": map[string]any{
				"source":  [][]any{{externalImage}},
				"caption": [][]any{{"External image"}},
			},
		},
		notionID: map[string]any{
			"id":   notionID,
			"type": "image",
			"properties": map[string]any{
				"source": [][]any{{"https://file.notion.so/workspace/leak.png?signature=secret"}},
			},
		},
		httpID: map[string]any{
			"id":   httpID,
			"type": "image",
			"properties": map[string]any{
				"source": [][]any{{"http://images.example.com/insecure.png"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-cover notion-media notion-media--full"><img src="https://images.example.com/cover.png" loading="lazy" decoding="async" alt=""></figure>`)
	assertContains(t, html, `<a class="notion-media__link" href="https://images.example.com/photo.png" data-collection-lightbox data-full-src="https://images.example.com/photo.png"><img src="https://images.example.com/photo.png"`)
	assertContains(t, html, `<figcaption>External image</figcaption>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "signature=secret") || strings.Contains(html, "http://images.example.com") {
		t.Fatalf("unsafe external image source leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageFallsBackUnknownSourceBlocksToBookmark(t *testing.T) {
	const (
		rootID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		unknownID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		textID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{unknownID, textID},
		},
		unknownID: map[string]any{
			"id":   unknownID,
			"type": `unknown widget"><script>`,
			"properties": map[string]any{
				"title": [][]any{{"Widget <title>"}},
			},
			"format": map[string]any{
				"display_source": "https://widgets.example.com/render?id=1",
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "mystery",
			"properties": map[string]any{
				"title": [][]any{{"Plain fallback"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-bookmark notion-bookmark--unknown-widgetscript"><a href="https://widgets.example.com/render?id=1"`)
	assertContains(t, html, `<strong>Widget &lt;title&gt;</strong>`)
	assertContains(t, html, `<span class="notion-bookmark__host">widgets.example.com</span>`)
	assertContains(t, html, `<p>Plain fallback</p>`)
	if strings.Contains(html, `<script`) || strings.Contains(html, `<title>`) {
		t.Fatalf("unknown source block leaked unsafe metadata: %s", html)
	}
}

func TestSafeEmbedURLSupportsAdditionalProviders(t *testing.T) {
	tests := []struct {
		name      string
		blockType string
		raw       string
		want      string
	}{
		{
			name:      "loom share",
			blockType: "embed",
			raw:       "https://www.loom.com/share/abc123?sid=secret",
			want:      "https://www.loom.com/embed/abc123",
		},
		{
			name:      "wistia media page",
			blockType: "video",
			raw:       "https://demo.wistia.com/medias/abc123_DEF?utm_source=notion",
			want:      "https://fast.wistia.net/embed/iframe/abc123_DEF",
		},
		{
			name:      "wistia iframe strips options",
			blockType: "embed",
			raw:       "https://fast.wistia.net/embed/iframe/abc123?seo=true&plugin%5Bfoo%5D%5Bbar%5D=x",
			want:      "https://fast.wistia.net/embed/iframe/abc123",
		},
		{
			name:      "tella video",
			blockType: "video",
			raw:       "https://www.tella.tv/video/abc-123_DEF?utm_source=notion",
			want:      "https://www.tella.tv/video/abc-123_DEF/embed",
		},
		{
			name:      "videoask remains fallback card",
			blockType: "video",
			raw:       "https://www.videoask.com/fabc123",
			want:      "",
		},
		{
			name:      "google drive file",
			blockType: "drive",
			raw:       "https://drive.google.com/file/d/1Ab_C-2/view?usp=sharing",
			want:      "https://drive.google.com/file/d/1Ab_C-2/preview",
		},
		{
			name:      "google docs spreadsheet",
			blockType: "drive",
			raw:       "https://docs.google.com/spreadsheets/d/sheet-id/edit#gid=0",
			want:      "https://docs.google.com/spreadsheets/d/sheet-id/preview",
		},
		{
			name:      "spotify already embedded",
			blockType: "embed",
			raw:       "https://open.spotify.com/embed/track/2S2ASICVK0L1LtjaysKsel",
			want:      "https://open.spotify.com/embed/track/2S2ASICVK0L1LtjaysKsel",
		},
		{
			name:      "google forms",
			blockType: "drive",
			raw:       "https://docs.google.com/forms/d/e/FORM_ID/viewform?usp=sf_link",
			want:      "https://docs.google.com/forms/d/e/FORM_ID/viewform?embedded=true",
		},
		{
			name:      "google maps place",
			blockType: "maps",
			raw:       "https://www.google.com/maps/place/Seoul/@37.5665,126.9780,12z",
			want:      "https://www.google.com/maps?output=embed&q=Seoul",
		},
		{
			name:      "soundcloud track",
			blockType: "embed",
			raw:       "https://soundcloud.com/artist/track",
			want:      "https://w.soundcloud.com/player/?auto_play=false&hide_related=true&show_comments=false&show_reposts=false&show_user=true&url=https%3A%2F%2Fsoundcloud.com%2Fartist%2Ftrack&visual=true",
		},
		{
			name:      "airtable share",
			blockType: "embed",
			raw:       "https://airtable.com/shrABC123",
			want:      "https://airtable.com/embed/shrABC123",
		},
		{
			name:      "typeform",
			blockType: "typeform",
			raw:       "https://form.typeform.com/to/abc123",
			want:      "https://form.typeform.com/to/abc123?typeform-embed=embed-widget",
		},
		{
			name:      "codepen",
			blockType: "codepen",
			raw:       "https://codepen.io/alice/pen/PNaGbb",
			want:      "https://codepen.io/alice/embed/PNaGbb?default-tab=result",
		},
		{
			name:      "replit",
			blockType: "replit",
			raw:       "https://replit.com/@alice/hello?lite=1#main.py",
			want:      "https://replit.com/@alice/hello?embed=true&lite=1",
		},
		{
			name:      "miro board",
			blockType: "miro",
			raw:       "https://miro.com/app/board/uXjVK/",
			want:      "https://miro.com/app/live-embed/uXjVK/",
		},
		{
			name:      "excalidraw room",
			blockType: "excalidraw",
			raw:       "https://excalidraw.com/#room=abc,secret",
			want:      "https://excalidraw.com/#room=abc,secret",
		},
		{
			name:      "excalidraw generic embed normalizes host",
			blockType: "embed",
			raw:       "http://www.excalidraw.com/?theme=light#json=abc",
			want:      "https://excalidraw.com/?theme=light#json=abc",
		},
		{
			name:      "gist",
			blockType: "gist",
			raw:       "https://gist.github.com/alice/abcdef123456",
			want:      "https://gist.github.com/alice/abcdef123456.pibb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := safeEmbedURL(tt.blockType, tt.raw); got != tt.want {
				t.Fatalf("safeEmbedURL(%q, %q) = %q, want %q", tt.blockType, tt.raw, got, tt.want)
			}
		})
	}
}

func TestRenderPageRendersDrivePropertiesAsStaticCard(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		driveID      = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		thumbnailURL = "https://file.notion.so/workspace/drive-thumb.png"
		iconURL      = "https://prod-files-secure.s3.us-west-2.amazonaws.com/workspace/drive-icon.png"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{driveID},
		},
		driveID: map[string]any{
			"id":   driveID,
			"type": "drive",
			"format": map[string]any{
				"drive_properties": map[string]any{
					"url":       "https://drive.google.com/file/d/1QqtwutJ9KC5gGitlHymkgVwJvR-l4n9S/view?usp=drivesdk",
					"title":     "Spec <Sheet>",
					"thumbnail": thumbnailURL,
					"icon":      iconURL,
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			driveID + ":drive-thumbnail": "/notion-assets/drive/thumb.png",
			iconURL:                      "/notion-assets/drive/icon.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-google-drive"><a class="notion-google-drive__link" href="https://drive.google.com/file/d/1QqtwutJ9KC5gGitlHymkgVwJvR-l4n9S/view?usp=drivesdk" rel="noopener noreferrer">`)
	assertContains(t, html, `<span class="notion-google-drive__preview"><img src="/notion-assets/drive/thumb.png" loading="lazy" decoding="async" alt=""></span>`)
	assertContains(t, html, `<span class="notion-google-drive__body"><strong>Spec &lt;Sheet&gt;</strong><span class="notion-google-drive__source"><img class="notion-google-drive__icon" src="/notion-assets/drive/icon.png" loading="lazy" decoding="async" alt=""><span>drive.google.com</span></span></span>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "prod-files-secure") || strings.Contains(html, "<iframe") {
		t.Fatalf("drive card leaked direct Notion asset URL or iframe: %s", html)
	}
}

func TestRenderPageAddsProviderClassForGenericEmbeds(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		embedID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{embedID},
		},
		embedID: map[string]any{
			"id":   embedID,
			"type": "embed",
			"format": map[string]any{
				"display_source": "https://www.loom.com/share/abc123?sid=secret",
				"block_width":    640,
				"block_height":   360,
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `class="notion-embed notion-embed--embed notion-embed--loom"`)
	assertContains(t, html, `style="--notion-embed-width:640px;--notion-embed-height:360px"`)
	assertContains(t, html, `src="https://www.loom.com/embed/abc123"`)
	assertContains(t, html, `href="https://www.loom.com/share/abc123?sid=secret"`)
	if strings.Contains(html, "5000px") || strings.Contains(html, "onmouseover") {
		t.Fatalf("unsafe embed format leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersCoreEmbedProvidersWithStableClasses(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		spotifyID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		mapsID       = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		figmaID      = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		soundcloudID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		airtableID   = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{spotifyID, mapsID, figmaID, soundcloudID, airtableID},
		},
		spotifyID: map[string]any{
			"id":   spotifyID,
			"type": "embed",
			"format": map[string]any{
				"display_source": "https://open.spotify.com/playlist/2MgkBl2rQnPdF3WZ6ciajc?si=secret",
			},
		},
		mapsID: map[string]any{
			"id":   mapsID,
			"type": "maps",
			"properties": map[string]any{
				"source": [][]any{{"https://www.google.com/maps/place/Seoul"}},
			},
		},
		figmaID: map[string]any{
			"id":   figmaID,
			"type": "figma",
			"properties": map[string]any{
				"source":  [][]any{{"https://www.figma.com/file/abc123/Design"}},
				"caption": [][]any{{"Figma <safe>"}},
			},
		},
		soundcloudID: map[string]any{
			"id":   soundcloudID,
			"type": "embed",
			"properties": map[string]any{
				"source": [][]any{{"https://soundcloud.com/artist/track"}},
			},
		},
		airtableID: map[string]any{
			"id":   airtableID,
			"type": "embed",
			"properties": map[string]any{
				"source": [][]any{{"https://airtable.com/shrABC123/tbl456"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-embed notion-embed--embed notion-embed--spotify"><iframe title="open.spotify.com" src="https://open.spotify.com/embed/playlist/2MgkBl2rQnPdF3WZ6ciajc"`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--maps"><iframe title="google.com" src="https://www.google.com/maps?output=embed&amp;q=Seoul"`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--figma"><iframe title="figma.com" src="https://www.figma.com/embed?embed_host=unofficial-notion-go&amp;url=https%3A%2F%2Fwww.figma.com%2Ffile%2Fabc123%2FDesign"`)
	assertContains(t, html, `<figcaption>Figma &lt;safe&gt;<br><a href="https://www.figma.com/file/abc123/Design" rel="noopener noreferrer">`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--embed notion-embed--soundcloud"><iframe title="soundcloud.com" src="https://w.soundcloud.com/player/?auto_play=false&amp;hide_related=true&amp;show_comments=false&amp;show_reposts=false&amp;show_user=true&amp;url=https%3A%2F%2Fsoundcloud.com%2Fartist%2Ftrack&amp;visual=true"`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--embed notion-embed--airtable"><iframe title="airtable.com" src="https://airtable.com/embed/shrABC123"`)
	if strings.Count(html, `sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation"`) != 5 {
		t.Fatalf("core embed providers did not render as sandboxed iframes: %s", html)
	}
	if strings.Contains(html, `src="https://open.spotify.com/embed/playlist/2MgkBl2rQnPdF3WZ6ciajc?`) {
		t.Fatalf("provider tracking params leaked into iframe src: %s", html)
	}
}

func TestRenderPageRendersAdditionalVideoProvidersAsSandboxedEmbeds(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		wistiaID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		tellaID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{wistiaID, tellaID},
		},
		wistiaID: map[string]any{
			"id":   wistiaID,
			"type": "video",
			"properties": map[string]any{
				"source": [][]any{{"https://demo.wistia.com/medias/abc123?tracking=1"}},
			},
		},
		tellaID: map[string]any{
			"id":   tellaID,
			"type": "video",
			"properties": map[string]any{
				"source": [][]any{{"https://www.tella.tv/video/demo-123?utm_source=notion"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-embed notion-embed--video notion-embed--wistia"><iframe title="demo.wistia.com" src="https://fast.wistia.net/embed/iframe/abc123"`)
	assertContains(t, html, `<figure class="notion-embed notion-embed--video notion-embed--tella"><iframe title="tella.tv" src="https://www.tella.tv/video/demo-123/embed"`)
	if strings.Count(html, `sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation"`) != 2 {
		t.Fatalf("additional video providers did not render as sandboxed iframes: %s", html)
	}
	if strings.Contains(html, `src="https://fast.wistia.net/embed/iframe/abc123?`) ||
		strings.Contains(html, `src="https://www.tella.tv/video/demo-123/embed?`) {
		t.Fatalf("provider tracking params leaked into iframe src: %s", html)
	}
}

func TestRenderPageRendersExcalidrawAsSandboxedEmbed(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		drawID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{drawID},
		},
		drawID: map[string]any{
			"id":   drawID,
			"type": "excalidraw",
			"properties": map[string]any{
				"source":  [][]any{{"http://www.excalidraw.com/?theme=light#json=abc"}},
				"caption": [][]any{{"Sketch <safe>"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-embed notion-embed--excalidraw"><iframe title="excalidraw.com" src="https://excalidraw.com/?theme=light#json=abc"`)
	assertContains(t, html, `sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation"`)
	assertContains(t, html, `<figcaption>Sketch &lt;safe&gt;<br><a href="http://www.excalidraw.com/?theme=light#json=abc" rel="noopener noreferrer">`)
	if strings.Contains(html, `src="http://www.excalidraw.com`) {
		t.Fatalf("iframe src did not normalize Excalidraw host: %s", html)
	}
}

func TestRenderPageRendersTweetsAsStaticCards(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tweetID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{tweetID},
		},
		tweetID: map[string]any{
			"id":   tweetID,
			"type": "tweet",
			"properties": map[string]any{
				"source": [][]any{{"https://x.com/alice/status/1234567890"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-tweet"><a href="https://x.com/alice/status/1234567890" rel="noopener noreferrer">`)
	assertContains(t, html, `<span class="notion-tweet__eyebrow">Tweet</span><strong>Tweet by @alice</strong>`)
	assertContains(t, html, `<span class="notion-tweet__meta">x.com · 1234567890</span>`)
	if strings.Contains(html, `<iframe`) || strings.Contains(html, `<script`) {
		t.Fatalf("tweet rendered active third-party content instead of a static card: %s", html)
	}
}

func TestRenderPageAppliesSafeMediaFormat(t *testing.T) {
	const (
		rootID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		imageID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		videoID     = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		bannerID    = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		ultraWideID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{imageID, videoID, bannerID, ultraWideID},
			"format": map[string]any{
				"page_cover_position": 0.42,
			},
		},
		imageID: map[string]any{
			"id":   imageID,
			"type": "image",
			"format": map[string]any{
				"block_width":     480,
				"block_alignment": "center",
				// Notion stores block_aspect_ratio as height/width: 16:9 is 0.5625.
				"block_aspect_ratio": 0.5625,
			},
		},
		videoID: map[string]any{
			"id":   videoID,
			"type": "video",
			"format": map[string]any{
				"block_full_width": true,
				"block_alignment":  `right" onmouseover="alert(1)`,
				"block_width":      5000,
			},
		},
		bannerID: map[string]any{
			"id":   bannerID,
			"type": "image",
			"format": map[string]any{
				"block_width": 1200,
				// Wide banner regression: 220/2500 stored as h/w.
				"block_aspect_ratio": 0.088,
			},
		},
		ultraWideID: map[string]any{
			"id":   ultraWideID,
			"type": "image",
			"format": map[string]any{
				"block_width": 320,
				// Below the raw h/w guard (> 0.05); the variable must be dropped.
				"block_aspect_ratio": 0.01,
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			rootID:      "/notion-assets/cover/cover.jpg",
			imageID:     "/notion-assets/image/image.png",
			videoID:     "/notion-assets/video/video.mp4",
			bannerID:    "/notion-assets/image/banner.png",
			ultraWideID: "/notion-assets/image/ultra-wide.png",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<figure class="notion-cover notion-media notion-media--full" style="--notion-cover-position-y:58.00%">`)
	// The CSS variable is consumed by the width/height aspect-ratio property,
	// so the stored h/w value must come out reciprocated.
	assertContains(t, html, `<figure class="notion-media notion-media--image notion-media--center" style="--notion-media-width:480px;--notion-media-aspect-ratio:1.778">`)
	assertContains(t, html, `<figure class="notion-media notion-media--image" style="--notion-media-width:1200px;--notion-media-aspect-ratio:11.36">`)
	assertContains(t, html, `<figure class="notion-media notion-media--image" style="--notion-media-width:320px">`)
	assertContains(t, html, `<a class="notion-media__link" href="/notion-assets/image/image.png" data-collection-lightbox data-full-src="/notion-assets/image/image.png"><img src="/notion-assets/image/image.png"`)
	assertContains(t, html, `<figure class="notion-media notion-media--video notion-media--full">`)
	if strings.Contains(html, `onmouseover`) || strings.Contains(html, `5000px`) {
		t.Fatalf("unsafe media format leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersLocalMediaLabelsSafely(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		audioID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		videoID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		pdfID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{audioID, videoID, pdfID},
		},
		audioID: map[string]any{
			"id":   audioID,
			"type": "audio",
			"properties": map[string]any{
				"source": [][]any{{"https://file.notion.so/workspace/%3CAudio%3E.mp3?signature=stale"}},
			},
		},
		videoID: map[string]any{
			"id":   videoID,
			"type": "video",
			"properties": map[string]any{
				"title":  [][]any{{"Clip <safe>"}},
				"source": [][]any{{"https://file.notion.so/workspace/video.mp4?signature=stale"}},
			},
		},
		pdfID: map[string]any{
			"id":   pdfID,
			"type": "pdf",
			"properties": map[string]any{
				"source": [][]any{{"https://file.notion.so/workspace/%3CDoc%3E.pdf?signature=stale"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			audioID: "/notion-assets/audio/audio.mp3",
			videoID: "/notion-assets/video/video.mp4",
			pdfID:   "/notion-assets/pdf/%3CDoc%3E.pdf",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<audio controls preload="metadata" src="/notion-assets/audio/audio.mp3" aria-label="&lt;Audio&gt;.mp3"></audio><figcaption>&lt;Audio&gt;.mp3</figcaption>`)
	assertContains(t, html, `<video controls preload="metadata" src="/notion-assets/video/video.mp4" aria-label="video.mp4"></video><figcaption>Clip &lt;safe&gt;</figcaption>`)
	assertContains(t, html, `<figcaption><a href="/notion-assets/pdf/%3CDoc%3E.pdf">&lt;Doc&gt;.pdf</a></figcaption>`)
	if strings.Contains(html, "file.notion.so") || strings.Contains(html, "<Audio>.mp3") || strings.Contains(html, "<Doc>.pdf") {
		t.Fatalf("rendered unsafe media label or Notion-hosted source: %s", html)
	}
}

// Video and audio elements always carry an accessible name: the caption text
// when present, otherwise the file label (mirroring image alt handling).
func TestRenderPageVideoAndAudioAlwaysCarryAriaLabel(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		videoID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		audioID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{videoID, audioID},
		},
		videoID: map[string]any{
			"id":   videoID,
			"type": "video",
			"properties": map[string]any{
				"source":  [][]any{{"https://file.notion.so/workspace/video.mp4?signature=stale"}},
				"caption": [][]any{{"Launch <recap>"}},
			},
		},
		audioID: map[string]any{
			"id":   audioID,
			"type": "audio",
			"properties": map[string]any{
				"source":  [][]any{{"https://file.notion.so/workspace/episode.mp3?signature=stale"}},
				"caption": [][]any{{"Episode one"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
		AssetURLs: map[string]string{
			videoID: "/notion-assets/video/video.mp4",
			audioID: "/notion-assets/audio/episode.mp3",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<video controls preload="metadata" src="/notion-assets/video/video.mp4" aria-label="Launch &lt;recap&gt;"></video>`)
	assertContains(t, html, `<audio controls preload="metadata" src="/notion-assets/audio/episode.mp3" aria-label="Episode one"></audio>`)
}
