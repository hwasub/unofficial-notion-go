// Command cli renders a public Notion page to a self-contained HTML file.
//
// It demonstrates the full library flow end to end: fetch a snapshot with
// ingest.FetchSnapshot, download each discovered asset into a local directory,
// map those assets to caller-controlled URLs, and render sanitized HTML with
// notion.RenderPage.
//
// Usage:
//
//	go run ./examples/cli -url "https://www.notion.so/Example-1ad6e61cf82480c9a6c4d251043457d3" -out ./out
//
// The generated out/index.html links out/notion.css and out/notion.js (copied
// from the -css and -js paths), so opening it in a browser shows the styled,
// interactive page.
//
// This is a demonstration, not production code: it has minimal error handling
// and downloads assets serially.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
)

func main() {
	pageURL := flag.String("url", "", "public Notion page URL or ID (required)")
	outDir := flag.String("out", "out", "output directory for index.html and assets")
	cssPath := flag.String("css", "examples/notion.css", "stylesheet to copy next to index.html (empty to skip)")
	jsPath := flag.String("js", "examples/notion.js", "script to copy next to index.html (empty to skip)")
	maxAssets := flag.Int("max-assets", 200, "maximum number of assets to fetch")
	flag.Parse()

	if strings.TrimSpace(*pageURL) == "" {
		flag.Usage()
		log.Fatal("the -url flag is required")
	}
	if err := run(*pageURL, *outDir, *cssPath, *jsPath, *maxAssets); err != nil {
		log.Fatal(err)
	}
}

func run(pageURL, outDir, cssPath, jsPath string, maxAssets int) error {
	ctx := context.Background()

	// 1. Resolve limits and fetch a snapshot. A nil FetchOptions.Client makes
	//    FetchSnapshot construct a real notionapi client for public pages.
	defaults := ingest.DefaultLimitsFromEnv()
	req := ingest.FetchRequest{URL: pageURL, MaxAssets: maxAssets}
	limits := ingest.RequestLimitsForRequest(req, defaults)

	snapshot, err := ingest.FetchSnapshot(ctx, req, limits, ingest.FetchOptions{})
	if err != nil {
		return fmt.Errorf("fetch snapshot: %w", err)
	}
	for _, e := range snapshot.Errors {
		log.Printf("warning: %s: %s", e.PageID, e.Message)
	}

	// 2. Download each discovered asset and build caller-controlled URLs. Signed
	//    Notion URLs are short-lived, so they must be downloaded now and served
	//    from your own storage rather than referenced directly in public HTML.
	assetsDir := filepath.Join(outDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return err
	}
	assetURLs, downloaded := downloadAssets(ctx, snapshot.Assets, assetsDir)

	// 3. Render sanitized HTML. The snapshot's normalized record map serializes
	//    directly into RenderInput.RecordMap.
	recordMap, err := json.Marshal(snapshot.Page.RecordMap)
	if err != nil {
		return err
	}
	body, err := notion.RenderPage(notion.RenderInput{
		RecordMap: recordMap,
		PageID:    snapshot.RootPageID,
		AssetURLs: assetURLs,
	})
	if err != nil {
		return fmt.Errorf("render page: %w", err)
	}

	// 4. Wrap the rendered body in a minimal HTML document and write everything.
	indexPath := filepath.Join(outDir, "index.html")
	if err := os.WriteFile(indexPath, []byte(document(snapshot.Page.Title, body, cssPath != "", jsPath != "")), 0o644); err != nil {
		return err
	}
	if cssPath != "" {
		if err := copyFile(cssPath, filepath.Join(outDir, "notion.css")); err != nil {
			log.Printf("could not copy stylesheet %s: %v", cssPath, err)
		}
	}
	if jsPath != "" {
		if err := copyFile(jsPath, filepath.Join(outDir, "notion.js")); err != nil {
			log.Printf("could not copy script %s: %v", jsPath, err)
		}
	}

	fmt.Printf("wrote %s (%d of %d assets downloaded)\n", indexPath, downloaded, len(snapshot.Assets))
	return nil
}

// downloadAssets fetches every asset's signed URL into dir and returns a map of
// caller-controlled URLs keyed by both block ID and source URL, as RenderPage
// expects, plus the number of assets successfully downloaded. Assets without a
// signed URL (signing failed upstream) are skipped.
func downloadAssets(ctx context.Context, assets []ingest.AssetSnapshot, dir string) (map[string]string, int) {
	client := &http.Client{Timeout: 30 * time.Second}
	urls := make(map[string]string, len(assets)*2)
	downloaded := 0
	for i, asset := range assets {
		if strings.TrimSpace(asset.SignedURL) == "" {
			log.Printf("skipping asset %s: no signed URL", asset.BlockID)
			continue
		}
		name := assetFilename(i, asset)
		if err := download(ctx, client, asset.SignedURL, filepath.Join(dir, name)); err != nil {
			log.Printf("skipping asset %s: %v", asset.BlockID, err)
			continue
		}
		public := path.Join("assets", name)
		urls[asset.BlockID] = public
		if asset.Source != "" {
			urls[asset.Source] = public
		}
		downloaded++
	}
	return urls, downloaded
}

func download(ctx context.Context, client *http.Client, src, dst string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// assetFilename derives a stable, collision-free file name for an asset.
func assetFilename(i int, asset ingest.AssetSnapshot) string {
	name := asset.Filename
	if name == "" {
		if u, err := url.Parse(asset.Source); err == nil {
			name = path.Base(u.Path)
		}
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.' || r == '-' || r == '_':
			return r
		default:
			return '-'
		}
	}, name)
	if name == "" || name == "-" {
		name = "asset"
	}
	return fmt.Sprintf("%03d-%s", i, name)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func document(title, body string, includeCSS bool, includeJS bool) string {
	head := []string{
		"<!DOCTYPE html>",
		`<html lang="en">`,
		"<head>",
		`<meta charset="utf-8">`,
		`<meta name="viewport" content="width=device-width, initial-scale=1">`,
		"<title>" + html.EscapeString(title) + "</title>",
	}
	if includeCSS {
		head = append(head, `<link rel="stylesheet" href="notion.css">`)
	}
	if includeJS {
		head = append(head, `<script defer src="notion.js"></script>`)
	}
	head = append(head,
		"</head>",
		"<body>",
		`<main class="notion-body-wrap" data-collection-lightbox-labels data-lightbox-previous="Previous" data-lightbox-next="Next" data-lightbox-close="Close" data-lightbox-label="Image viewer">`,
		body,
		"</main>",
		"</body>",
		"</html>",
		"",
	)
	return strings.Join(head, "\n")
}
