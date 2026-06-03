package notion

import "testing"

func TestRenderPageCachedStoresRenderedHTMLOnDisk(t *testing.T) {
	initRenderCache(1<<20, t.TempDir())
	t.Cleanup(func() { initRenderCache(0, "") })

	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	input := RenderInput{
		RecordMap: marshalRecordMap(t, map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{textID},
			},
			textID: map[string]any{
				"id":   textID,
				"type": "text",
				"properties": map[string]any{
					"title": [][]any{{"Cached body"}},
				},
			},
		}),
		PageID: rootID,
	}

	first, err := RenderPageCached(input, "en")
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderPageCached(input, "en")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("cached render changed: %q != %q", first, second)
	}
	stats := RenderCacheStats()
	if stats.Entries != 1 || stats.Misses != 1 || stats.Hits != 1 {
		t.Fatalf("stats = %+v, want one entry, one miss, one hit", stats)
	}
}

func TestRenderCacheKeyIncludesLocale(t *testing.T) {
	input := RenderInput{PageID: "page", RecordMap: []byte(`{"block":{}}`)}
	if RenderCacheKey(input, "en") == RenderCacheKey(input, "ko") {
		t.Fatal("RenderCacheKey did not include locale")
	}
}
