package notion

import (
	"errors"
	"testing"
)

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

func TestRenderCacheKeyIncludesLocalizationVersion(t *testing.T) {
	input := RenderInput{PageID: "page", RecordMap: []byte(`{"block":{}}`)}
	withVersion := input
	withVersion.LocalizationVersion = "fr-v2"
	if RenderCacheKey(input, "en") == RenderCacheKey(withVersion, "en") {
		t.Fatal("RenderCacheKey did not include LocalizationVersion")
	}
}

func TestRenderCacheKeyIncludesExplicitPageURLs(t *testing.T) {
	input := RenderInput{PageID: "page", RecordMap: []byte(`{"block":{}}`)}
	withURL := input
	withURL.PageURLs = map[string]string{"page": "/render?id=page"}
	if RenderCacheKey(input, "en") == RenderCacheKey(withURL, "en") {
		t.Fatal("RenderCacheKey did not include PageURLs")
	}
}

func TestRenderCacheKeyIncludesUnsafeFlag(t *testing.T) {
	input := RenderInput{PageID: "page", RecordMap: []byte(`{"block":{}}`)}
	unsafe := input
	unsafe.UnsafeRenderNotionSignedURLs = true
	if RenderCacheKey(input, "en") == RenderCacheKey(unsafe, "en") {
		t.Fatal("RenderCacheKey did not include UnsafeRenderNotionSignedURLs")
	}
}

func TestRenderCacheKeyIncludesEffectiveRenderLimits(t *testing.T) {
	input := RenderInput{PageID: "page", RecordMap: []byte(`{"block":{}}`)}
	explicitDefaults := input
	explicitDefaults.MaxRecordMapBytes = DefaultMaxRecordMapBytes
	explicitDefaults.MaxBlocks = DefaultMaxBlocks
	explicitDefaults.MaxOutputBytes = DefaultMaxOutputBytes
	if RenderCacheKey(input, "en") != RenderCacheKey(explicitDefaults, "en") {
		t.Fatal("zero and explicit default limits produced different cache keys")
	}

	tests := []struct {
		name   string
		change func(*RenderInput)
	}{
		{"record_map", func(in *RenderInput) { in.MaxRecordMapBytes = 1 }},
		{"blocks", func(in *RenderInput) { in.MaxBlocks = 1 }},
		{"output", func(in *RenderInput) { in.MaxOutputBytes = 1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strict := input
			tt.change(&strict)
			if RenderCacheKey(input, "en") == RenderCacheKey(strict, "en") {
				t.Fatalf("RenderCacheKey did not include effective %s limit", tt.name)
			}
		})
	}
}

func TestRenderPageCachedDoesNotBypassStricterRenderLimits(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	base := RenderInput{
		RecordMap: marshalRecordMap(t, map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{textID},
			},
			textID: map[string]any{
				"id":         textID,
				"type":       "text",
				"properties": map[string]any{"title": [][]any{{"Cached body"}}},
			},
		}),
		PageID:            rootID,
		MaxRecordMapBytes: -1,
		MaxBlocks:         -1,
		MaxOutputBytes:    -1,
	}

	tests := []struct {
		name   string
		strict func(*RenderInput)
		want   error
	}{
		{"record_map", func(in *RenderInput) { in.MaxRecordMapBytes = 1 }, ErrRecordMapTooLarge},
		{"blocks", func(in *RenderInput) { in.MaxBlocks = 1 }, ErrTooManyBlocks},
		{"output", func(in *RenderInput) { in.MaxOutputBytes = 1 }, ErrOutputTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initRenderCache(1<<20, t.TempDir())
			t.Cleanup(func() { initRenderCache(0, "") })
			if _, err := RenderPageCached(base, "en"); err != nil {
				t.Fatalf("populate permissive cache: %v", err)
			}
			strict := base
			tt.strict(&strict)
			if _, err := RenderPageCached(strict, "en"); !errors.Is(err, tt.want) {
				t.Fatalf("strict cached render error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRenderPageCachedBypassesCacheForUnsafeURLs(t *testing.T) {
	initRenderCache(1<<20, t.TempDir())
	t.Cleanup(func() { initRenderCache(0, "") })

	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	input := RenderInput{
		RecordMap: marshalRecordMap(t, map[string]any{
			rootID: map[string]any{
				"id":   rootID,
				"type": "page",
			},
		}),
		PageID:                       rootID,
		UnsafeRenderNotionSignedURLs: true,
	}

	if _, err := RenderPageCached(input, "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := RenderPageCached(input, "en"); err != nil {
		t.Fatal(err)
	}

	stats := RenderCacheStats()
	if stats.Entries != 0 || stats.Hits != 0 || stats.Misses != 0 {
		t.Fatalf("stats = %+v, want no cache activity for unsafe renders", stats)
	}
}
