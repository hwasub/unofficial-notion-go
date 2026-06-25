// Command server renders public Notion pages over HTTP and proxies their
// assets from memory, demonstrating the caller-controlled asset-serving pattern
// the library expects.
//
// Usage:
//
//	go run ./examples/server -addr :8080 -js examples/notion.js
//
// Then open http://localhost:8080/ and paste a public Notion page URL. The
// stylesheet served at /notion.css defaults to the embedded notion.StyleCSS();
// pass -css to serve a custom file instead, or -css none to skip it.
//
// This is a demonstration, not production code: assets are cached in memory and
// never evicted, non-media files are served as downloads, there is no
// authentication or rate limiting, and every request re-fetches the page from
// Notion.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
)

// defaultMaxAssetBytes caps a single proxied asset. Larger downloads are
// rejected rather than silently truncated to a corrupt file.
const defaultMaxAssetBytes = 64 << 20

// Asset budgets bounding memory use. defaultMaxRequestAssetBytes caps the total
// bytes one /render call will download; defaultMaxStoreBytes caps the whole
// in-memory asset store across requests (this demo never evicts, so it refuses
// new assets once full). Operators should tune these for real traffic.
const (
	defaultMaxRequestAssetBytes = 256 << 20
	defaultMaxStoreBytes        = 512 << 20
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	cssPath := flag.String("css", "", "stylesheet served at /notion.css (default: embedded notion.StyleCSS(); \"none\" to skip)")
	jsPath := flag.String("js", "examples/notion.js", "script served at /notion.js")
	flag.Parse()

	srv := &server{
		assets:               map[string][]byte{},
		client:               &http.Client{Timeout: 30 * time.Second},
		cssPath:              *cssPath,
		jsPath:               *jsPath,
		maxAssetBytes:        defaultMaxAssetBytes,
		maxRequestAssetBytes: defaultMaxRequestAssetBytes,
		maxStoreBytes:        defaultMaxStoreBytes,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.handleIndex)
	mux.HandleFunc("/render", srv.handleRender)
	mux.HandleFunc("/assets/", srv.handleAsset)
	if *cssPath != "none" {
		mux.HandleFunc("/notion.css", srv.handleCSS)
	}
	if *jsPath != "" {
		mux.HandleFunc("/notion.js", srv.handleJS)
	}

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

type server struct {
	mu                   sync.Mutex
	assets               map[string][]byte
	usedBytes            int64 // total bytes currently held in assets
	client               *http.Client
	cssPath              string
	jsPath               string
	maxAssetBytes        int64 // per-asset download cap; <=0 uses defaultMaxAssetBytes
	maxRequestAssetBytes int64 // per-/render total download cap; <=0 uses default
	maxStoreBytes        int64 // whole in-memory store cap; <=0 uses default
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">`+
		`<title>Notion renderer demo</title></head><body>`+
		`<h1>Notion renderer demo</h1>`+
		`<form action="/render"><input name="url" size="80" `+
		`placeholder="https://www.notion.so/..."> <button>Render</button></form>`+
		`</body></html>`)
}

func (s *server) handleRender(w http.ResponseWriter, r *http.Request) {
	pageURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if pageURL == "" {
		http.Error(w, "missing url query parameter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	defaults := ingest.DefaultLimitsFromEnv()
	req := ingest.FetchRequest{URL: pageURL, MaxAssets: 200}
	limits := ingest.RequestLimitsForRequest(req, defaults)

	snapshot, err := ingest.FetchSnapshot(ctx, req, limits, ingest.FetchOptions{})
	if err != nil {
		http.Error(w, "fetch: "+err.Error(), http.StatusBadGateway)
		return
	}

	// Download assets into the in-memory store and key the public /assets/<id>
	// URLs by both block ID and source URL, as RenderPage expects.
	assetURLs := s.proxyAssets(ctx, snapshot.Assets)

	recordMap, err := json.Marshal(snapshot.Page.RecordMap)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err := notion.RenderPage(notion.RenderInput{
		RecordMap: recordMap,
		PageID:    snapshot.RootPageID,
		AssetURLs: assetURLs,
	})
	if err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, document(snapshot.Page.Title, body, s.cssPath != "none", s.jsPath != ""))
}

func (s *server) proxyAssets(ctx context.Context, assets []ingest.AssetSnapshot) map[string]string {
	urls := make(map[string]string, len(assets)*2)
	budget := s.maxRequestAssetBytes
	if budget <= 0 {
		budget = defaultMaxRequestAssetBytes
	}
	var used int64
	for _, asset := range assets {
		if strings.TrimSpace(asset.SignedURL) == "" {
			continue
		}
		if used >= budget {
			log.Printf("asset %s: per-request asset budget of %d bytes reached, skipping the rest", asset.BlockID, budget)
			break
		}
		data, err := s.fetch(ctx, asset.SignedURL)
		if err != nil {
			log.Printf("asset %s: %v", asset.BlockID, err)
			continue
		}
		used += int64(len(data))
		key := hashKey(asset.BlockID, asset.Source)
		if !s.store(key, data) {
			log.Printf("asset %s: in-memory asset store is full, skipping", asset.BlockID)
			continue
		}
		public := "/assets/" + key
		urls[asset.BlockID] = public
		if asset.Source != "" {
			urls[asset.Source] = public
		}
	}
	return urls
}

// store records an asset unless doing so would exceed the in-memory store cap.
// It returns whether the asset is available afterward (already present or newly
// stored). The demo never evicts, so it refuses new assets once full.
func (s *server) store(key string, data []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.assets[key]; ok {
		return true
	}
	limit := s.maxStoreBytes
	if limit <= 0 {
		limit = defaultMaxStoreBytes
	}
	if s.usedBytes+int64(len(data)) > limit {
		return false
	}
	s.assets[key] = data
	s.usedBytes += int64(len(data))
	return true
}

func (s *server) fetch(ctx context.Context, src string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	limit := s.maxAssetBytes
	if limit <= 0 {
		limit = defaultMaxAssetBytes
	}
	// Read one extra byte so an asset exactly at the limit is accepted but an
	// oversized one is rejected instead of being silently truncated.
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("asset exceeds %d bytes", limit)
	}
	return data, nil
}

func (s *server) handleAsset(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/assets/")
	s.mu.Lock()
	data, ok := s.assets[key]
	s.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	contentType, inline := safeAssetContentType(data)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !inline {
		w.Header().Set("Content-Disposition", "attachment")
	}
	_, _ = w.Write(data)
}

func (s *server) handleCSS(w http.ResponseWriter, r *http.Request) {
	if s.cssPath == "" {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		io.WriteString(w, notion.StyleCSS())
		return
	}
	http.ServeFile(w, r, s.cssPath)
}

func (s *server) handleJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	http.ServeFile(w, r, s.jsPath)
}

func safeAssetContentType(data []byte) (string, bool) {
	detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(data)))
	switch {
	case strings.HasPrefix(detected, "image/") && detected != "image/svg+xml":
		return detected, true
	case strings.HasPrefix(detected, "audio/"), strings.HasPrefix(detected, "video/"):
		return detected, true
	case detected == "application/pdf":
		return detected, false
	default:
		return "application/octet-stream", false
	}
}

func hashKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:16]
}

func document(title, body string, includeCSS bool, includeJS bool) string {
	out := `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">` +
		`<meta name="viewport" content="width=device-width, initial-scale=1">` +
		"<title>" + html.EscapeString(title) + "</title>"
	if includeCSS {
		out += `<link rel="stylesheet" href="/notion.css">`
	}
	if includeJS {
		out += `<script defer src="/notion.js"></script>`
	}
	out += `</head><body>` +
		`<main class="notion-body-wrap" data-collection-lightbox-labels data-lightbox-previous="Previous" data-lightbox-next="Next" data-lightbox-close="Close" data-lightbox-label="Image viewer">` +
		body + `</main></body></html>`
	return out
}
