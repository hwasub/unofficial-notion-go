package main

import (
	"strings"
	"testing"
)

func TestSafeAssetContentTypeOnlyInlinesPassiveMedia(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		wantType   string
		wantInline bool
	}{
		{
			name:       "png",
			data:       []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0},
			wantType:   "image/png",
			wantInline: true,
		},
		{
			name:       "html",
			data:       []byte("<!doctype html><script>alert(1)</script>"),
			wantType:   "application/octet-stream",
			wantInline: false,
		},
		{
			name:       "svg",
			data:       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			wantType:   "application/octet-stream",
			wantInline: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType, gotInline := safeAssetContentType(tc.data)
			if gotType != tc.wantType || gotInline != tc.wantInline {
				t.Fatalf("safeAssetContentType = %q, %v; want %q, %v", gotType, gotInline, tc.wantType, tc.wantInline)
			}
		})
	}
}

func TestDocumentIncludesOptionalAssets(t *testing.T) {
	html := document("Test", "<p>body</p>", true, true)
	if !strings.Contains(html, `<link rel="stylesheet" href="/notion.css">`) {
		t.Fatalf("document missing stylesheet link: %s", html)
	}
	if !strings.Contains(html, `<script defer src="/notion.js"></script>`) {
		t.Fatalf("document missing script link: %s", html)
	}
	if !strings.Contains(html, `data-collection-lightbox-labels`) {
		t.Fatalf("document missing lightbox labels: %s", html)
	}

	html = document("Test", "<p>body</p>", false, false)
	if strings.Contains(html, "notion.css") || strings.Contains(html, "notion.js") {
		t.Fatalf("document included disabled assets: %s", html)
	}
}
