package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hwasub/unofficial-notion-go/ingest"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abc123")

	if err := writeFileAtomic(path, []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "hello" {
		t.Fatalf("content = %q, want hello", got)
	}

	// Overwriting replaces the file and leaves no temp files behind.
	if err := writeFileAtomic(path, []byte("world!!")); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "world!!" {
		t.Fatalf("content = %q, want world!!", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("expected exactly one file, got %d (temp left behind?)", len(entries))
	}
}

func TestRenderPageURLsUseInternalIDRoute(t *testing.T) {
	const pageID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	got := renderPageURLs(map[string]string{pageID: pageID})
	if got[pageID] != "/render?id="+pageID || strings.Contains(got[pageID], "notion.so") {
		t.Fatalf("render page URL = %q, want internal ID route", got[pageID])
	}
}

func TestRenderFetchRequestAcceptsInternalPageID(t *testing.T) {
	const pageID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	req, ok := renderFetchRequest(httptest.NewRequest("GET", "/render?id="+pageID, nil), 23)
	if !ok || req.PageID != pageID || req.URL != "" || req.MaxAssets != 23 {
		t.Fatalf("render fetch request = %+v, %v", req, ok)
	}
}

func TestEmbeddedScriptEnhancesServerRenderedTabs(t *testing.T) {
	script, err := embeddedStatic.ReadFile("static/notion.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(script)
	for _, want := range []string{
		"initializeNotionTabs();",
		`root.classList.add("notion-tabs--enhanced")`,
		"panel.hidden = panel.id !== targetID",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("embedded tab script missing %q", want)
		}
	}
}

func TestStoreAssetsRespectsTotalBudget(t *testing.T) {
	payload := make([]byte, 1000)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer upstream.Close()

	// 500-byte budget with 1000-byte assets: only the first download fits.
	a, err := newApp(t.TempDir(), 10, 500)
	if err != nil {
		t.Fatal(err)
	}
	assets := []ingest.AssetSnapshot{
		{BlockID: "b1", Source: "s1", SignedURL: upstream.URL},
		{BlockID: "b2", Source: "s2", SignedURL: upstream.URL},
		{BlockID: "b3", Source: "s3", SignedURL: upstream.URL},
	}
	urls, errCount := a.storeAssets(context.Background(), assets)

	if _, ok := urls["b1"]; !ok {
		t.Fatal("first asset should be stored within budget")
	}
	if _, ok := urls["b2"]; ok {
		t.Fatal("assets past the budget should be skipped")
	}
	if errCount == 0 {
		t.Fatal("expected a non-zero error count so the truncation surfaces as a warning")
	}
}
