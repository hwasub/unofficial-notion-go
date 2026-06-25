package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hwasub/unofficial-notion-go/notion"
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

func TestServerFetchRejectsOversizedAsset(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 64))
	}))
	defer upstream.Close()

	s := &server{client: upstream.Client(), maxAssetBytes: 16}
	if _, err := s.fetch(context.Background(), upstream.URL); err == nil {
		t.Fatal("expected oversized asset to be rejected, got nil error")
	}
}

func TestServerFetchAcceptsAssetWithinLimit(t *testing.T) {
	payload := make([]byte, 16)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer upstream.Close()

	s := &server{client: upstream.Client(), maxAssetBytes: 16}
	data, err := s.fetch(context.Background(), upstream.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != len(payload) {
		t.Fatalf("got %d bytes, want %d", len(data), len(payload))
	}
}

func TestServerStoreEnforcesBudget(t *testing.T) {
	s := &server{assets: map[string][]byte{}, maxStoreBytes: 10}
	if !s.store("a", make([]byte, 6)) {
		t.Fatal("first store within budget should succeed")
	}
	if s.store("b", make([]byte, 6)) {
		t.Fatal("second store exceeding the budget should be refused")
	}
	if _, ok := s.assets["b"]; ok {
		t.Fatal("refused asset must not be stored")
	}
	// Re-storing an existing key is a no-op and stays available.
	if !s.store("a", make([]byte, 6)) {
		t.Fatal("existing key should remain available")
	}
}

func TestHandleCSSDefaultsToEmbeddedStylesheet(t *testing.T) {
	srv := &server{}
	recorder := httptest.NewRecorder()
	srv.handleCSS(recorder, httptest.NewRequest("GET", "/notion.css", nil))
	if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/css") {
		t.Fatalf("content type = %q", got)
	}
	if recorder.Body.String() != notion.StyleCSS() {
		t.Fatal("embedded stylesheet not served verbatim")
	}
}
