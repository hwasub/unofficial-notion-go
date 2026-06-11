// Command fullapp is a small public Notion renderer starter app.
package main

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
)

const (
	maxAssetBytes = 64 << 20
	katexCSSURL   = "https://cdn.jsdelivr.net/npm/katex@0.17.0/dist/katex.min.css"
	katexJSURL    = "https://cdn.jsdelivr.net/npm/katex@0.17.0/dist/katex.min.js"
)

//go:embed static
var embeddedStatic embed.FS

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	assetDir := flag.String("asset-dir", "", "directory for downloaded Notion assets; default: a temporary directory")
	maxAssets := flag.Int("max-assets", 200, "maximum Notion assets to discover and proxy per render")
	flag.Parse()

	static, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		log.Fatal(err)
	}

	app, err := newApp(*assetDir, *maxAssets)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("asset directory: %s", app.assetDir)

	mux := http.NewServeMux()
	mux.HandleFunc("/", app.handleIndex)
	mux.HandleFunc("/render", app.handleRender)
	mux.HandleFunc("/assets/", app.handleAsset)
	mux.Handle("/static/", cacheStatic(http.StripPrefix("/static/", http.FileServer(http.FS(static)))))
	// The renderer stylesheet comes from the library itself so it always
	// matches the HTML this build emits; the exact pattern wins over /static/.
	mux.Handle("/static/notion.css", cacheStatic(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		io.WriteString(w, notion.StyleCSS())
	})))

	log.Printf("listening on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

type app struct {
	assetDir  string
	client    *http.Client
	maxAssets int
}

func newApp(assetDir string, maxAssets int) (*app, error) {
	if maxAssets <= 0 {
		maxAssets = 200
	}
	if strings.TrimSpace(assetDir) == "" {
		dir, err := os.MkdirTemp("", "unofficial-notion-go-assets-*")
		if err != nil {
			return nil, err
		}
		assetDir = dir
	}
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return nil, err
	}
	return &app{
		assetDir:  assetDir,
		client:    &http.Client{Timeout: 30 * time.Second},
		maxAssets: maxAssets,
	}, nil
}

func (a *app) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeHTML(w, document("Notion renderer starter", "", homeBody("")))
}

func (a *app) handleRender(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pageURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if pageURL == "" {
		writeHTML(w, document("Notion renderer starter", "", homeBody("Enter a public Notion page URL.")))
		return
	}

	defaults := ingest.DefaultLimitsFromEnv()
	req := ingest.FetchRequest{URL: pageURL, MaxAssets: a.maxAssets}
	limits := ingest.RequestLimitsForRequest(req, defaults)

	snapshot, err := ingest.FetchSnapshot(r.Context(), req, limits, ingest.FetchOptions{})
	if err != nil {
		http.Error(w, "fetch: "+err.Error(), http.StatusBadGateway)
		return
	}

	assetURLs, assetErrors := a.storeAssets(r.Context(), snapshot.Assets)

	recordMap, err := json.Marshal(snapshot.Page.RecordMap)
	if err != nil {
		http.Error(w, "marshal record map: "+err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := notion.RenderPage(notion.RenderInput{
		RecordMap: recordMap,
		PageID:    snapshot.RootPageID,
		AssetURLs: assetURLs,
		Warnings:  warningsForSnapshot(snapshot, assetErrors),
	})
	if err != nil {
		http.Error(w, "render: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeHTML(w, document(firstNonEmpty(snapshot.Page.Title, "Rendered Notion page"), pageURL, body))
}

func (a *app) storeAssets(ctx context.Context, assets []ingest.AssetSnapshot) (map[string]string, int) {
	urls := make(map[string]string, len(assets)*3)
	var errors int
	for _, asset := range assets {
		signedURL := strings.TrimSpace(asset.SignedURL)
		if signedURL == "" {
			errors++
			log.Printf("asset %s: missing signed URL", asset.BlockID)
			continue
		}
		data, err := a.fetchAsset(ctx, signedURL)
		if err != nil {
			errors++
			log.Printf("asset %s: %v", asset.BlockID, err)
			continue
		}

		key := hashKey(asset.BlockID, asset.Source)
		if err := os.WriteFile(filepath.Join(a.assetDir, key), data, 0o644); err != nil {
			errors++
			log.Printf("asset %s: write: %v", asset.BlockID, err)
			continue
		}

		publicURL := "/assets/" + key
		if asset.BlockID != "" {
			urls[asset.BlockID] = publicURL
		}
		for _, source := range sourceURLKeys(asset.Source) {
			urls[source] = publicURL
		}
	}
	return urls, errors
}

func (a *app) fetchAsset(ctx context.Context, src string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxAssetBytes {
		return nil, fmt.Errorf("asset exceeds %d MiB", maxAssetBytes>>20)
	}
	return data, nil
}

func (a *app) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key := strings.TrimPrefix(r.URL.Path, "/assets/")
	if !validAssetKey(key) {
		http.NotFound(w, r)
		return
	}
	data, err := os.ReadFile(filepath.Join(a.assetDir, key))
	if errors.Is(err, os.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	contentType, inline := safeAssetContentType(data)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !inline {
		w.Header().Set("Content-Disposition", "attachment")
	}
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(data)
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

func writeHTML(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, html)
}

func homeBody(message string) string {
	var out strings.Builder
	out.WriteString(`<section class="fullapp-home"><h1>Notion renderer starter</h1>`)
	out.WriteString(`<form class="fullapp-form" action="/render" method="get">`)
	out.WriteString(`<label for="notion-url">Public Notion URL</label>`)
	out.WriteString(`<div class="fullapp-form__row"><input id="notion-url" name="url" type="url" autocomplete="url" placeholder="https://www.notion.so/..." required>`)
	out.WriteString(`<button type="submit">Render</button></div></form>`)
	if message != "" {
		out.WriteString(`<p class="fullapp-message">`)
		out.WriteString(html.EscapeString(message))
		out.WriteString(`</p>`)
	}
	out.WriteString(`</section>`)
	return out.String()
}

func document(title, currentURL, body string) string {
	escapedTitle := html.EscapeString(firstNonEmpty(title, "Rendered Notion page"))
	var out strings.Builder
	out.WriteString(`<!DOCTYPE html><html lang="en"><head><meta charset="utf-8">`)
	out.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	out.WriteString(`<title>`)
	out.WriteString(escapedTitle)
	out.WriteString(`</title>`)
	out.WriteString(`<link rel="stylesheet" href="/static/notion.css">`)
	out.WriteString(`<link rel="stylesheet" href="/static/fullapp.css">`)
	out.WriteString(`<link rel="stylesheet" href="`)
	out.WriteString(katexCSSURL)
	out.WriteString(`">`)
	out.WriteString(`<script defer src="`)
	out.WriteString(katexJSURL)
	out.WriteString(`"></script>`)
	out.WriteString(`<script defer src="/static/notion.js"></script>`)
	out.WriteString(`</head><body><header class="fullapp-topbar">`)
	out.WriteString(`<a class="fullapp-brand" href="/">unofficial-notion-go</a>`)
	out.WriteString(`<form class="fullapp-topbar__form" action="/render" method="get">`)
	out.WriteString(`<label class="fullapp-visually-hidden" for="topbar-url">Public Notion URL</label>`)
	out.WriteString(`<input id="topbar-url" name="url" type="url" autocomplete="url" value="`)
	out.WriteString(html.EscapeString(currentURL))
	out.WriteString(`" placeholder="https://www.notion.so/...">`)
	out.WriteString(`<button type="submit">Render</button></form></header>`)
	out.WriteString(`<main class="notion-body-wrap" data-collection-lightbox-labels data-lightbox-previous="Previous" data-lightbox-next="Next" data-lightbox-close="Close" data-lightbox-label="Image viewer">`)
	out.WriteString(body)
	out.WriteString(`</main></body></html>`)
	return out.String()
}

func warningsForSnapshot(snapshot *ingest.Snapshot, assetErrors int) []notion.RenderWarning {
	if snapshot == nil {
		return nil
	}
	warnings := make([]notion.RenderWarning, 0, 4)
	if len(snapshot.Errors) > 0 {
		warnings = append(warnings, notion.RenderWarning{Kind: notion.RenderWarningFetchErrors, Count: len(snapshot.Errors)})
	}
	if assetErrors > 0 {
		warnings = append(warnings, notion.RenderWarning{Kind: notion.RenderWarningAssetDownloadErrors, Count: assetErrors})
	}
	if snapshot.Truncated.Assets {
		warnings = append(warnings, notion.RenderWarning{Kind: notion.RenderWarningAssetsTruncated, Count: 1})
	}
	if snapshot.Truncated.Blocks {
		warnings = append(warnings, notion.RenderWarning{Kind: notion.RenderWarningBlocksTruncated, Count: 1})
	}
	return warnings
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

func validAssetKey(key string) bool {
	if len(key) != 16 {
		return false
	}
	for _, ch := range key {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return false
		}
	}
	return true
}

func sourceURLKeys(source string) []string {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}
	keys := []string{source}
	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return keys
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	withoutQuery := parsed.String()
	if withoutQuery != source {
		keys = append(keys, withoutQuery)
	}
	return keys
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
