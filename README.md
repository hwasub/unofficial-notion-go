# unofficial-notion-go

[![Go Reference](https://pkg.go.dev/badge/github.com/hwasub/unofficial-notion-go.svg)](https://pkg.go.dev/github.com/hwasub/unofficial-notion-go)
[![CI](https://github.com/hwasub/unofficial-notion-go/actions/workflows/ci.yml/badge.svg)](https://github.com/hwasub/unofficial-notion-go/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Fetch public Notion pages and render them as safe, static HTML from Go.

The module has zero third-party dependencies and uses only the Go standard
library. It provides the pieces needed to:

- fetch a public page through Notion's internal web API;
- normalize the response into a stable snapshot;
- discover images and files for download to caller-controlled storage;
- render sanitized HTML with page navigation and database views; and
- serve a version-matched, embedded stylesheet.

## Status

Experimental and **pre-1.0**: the public API may change between minor versions
until a v1.0.0 release. Pin the module version and read the
[changelog](#changelog) before upgrading.

This project is not affiliated with Notion and does not use Notion's official
REST API at `api.notion.com`. It calls internal endpoints under
`www.notion.so/api/v3`, which may change or stop working without notice.

Review Notion's current terms and policies before using this library in a
product. Callers are responsible for permissions, rate limiting, data handling,
and compliance with Notion's terms.

## Quick start

### Requirements

- Go 1.25 or newer.
- A public Notion page URL. Confirm that the page opens in a private browser
  window without signing in.
- Network access to `www.notion.so`.

### Try the starter app

The quickest way to see the complete flow is to run the included starter app.
It fetches a page, downloads its assets, serves the embedded stylesheet, and
keeps linked pages navigable:

```sh
git clone https://github.com/hwasub/unofficial-notion-go.git
cd unofficial-notion-go
go run ./examples/fullapp -addr :8080
```

Open <http://localhost:8080/>, paste a public page URL, and select **Render**.
The starter app is intentionally small and is suitable as a reference, not as a
production deployment.

### Add the module to your application

Install the current release explicitly:

```sh
go get github.com/hwasub/unofficial-notion-go@v0.1.3
```

The following program fetches one page, renders its text and external content,
and writes `page.html` together with the matching `notion.css`:

```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run . <public-notion-page-url>")
	}

	req := ingest.FetchRequest{
		URL:       os.Args[1],
		MaxAssets: 200,
	}
	limits := ingest.RequestLimitsForRequest(
		req,
		ingest.DefaultLimitsFromEnv(),
	)
	snapshot, err := ingest.FetchSnapshot(
		context.Background(),
		req,
		limits,
		ingest.FetchOptions{},
	)
	if err != nil {
		log.Fatal(err)
	}

	recordMap, err := json.Marshal(snapshot.Page.RecordMap)
	if err != nil {
		log.Fatal(err)
	}
	renderedHTML, err := notion.RenderPage(notion.RenderInput{
		RecordMap: recordMap,
		PageID:    snapshot.RootPageID,
	})
	if err != nil {
		log.Fatal(err)
	}

	document := `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <link rel="stylesheet" href="notion.css">
</head>
<body><main class="notion-body-wrap">` + renderedHTML + `</main></body>
</html>`

	if err := os.WriteFile("page.html", []byte(document), 0o644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("notion.css", []byte(notion.StyleCSS()), 0o644); err != nil {
		log.Fatal(err)
	}
}
```

Run it with:

```sh
go run . "https://www.notion.so/<public-page>"
```

For safety, Notion-hosted images and files are not emitted until you download
them and supply caller-controlled URLs through `RenderInput.AssetURLs`. See
[Asset handling](#asset-handling), or use the starter app for a complete,
runnable implementation.

## Packages

Most applications should start with `ingest` and `notion`:

| Package | Use it for |
| --- | --- |
| [`ingest`](https://pkg.go.dev/github.com/hwasub/unofficial-notion-go/ingest) | Fetching and normalizing a page, repairing collection data, discovering assets, and applying request limits. |
| [`notion`](https://pkg.go.dev/github.com/hwasub/unofficial-notion-go/notion) | Rendering snapshots as sanitized HTML, building page links, validating stored snapshots, and serving the embedded CSS. |
| [`notionapi`](https://pkg.go.dev/github.com/hwasub/unofficial-notion-go/notionapi) | Direct access to low-level internal endpoints when the high-level snapshot workflow is not enough. |

Internal helpers — ID parsing, the byte-budgeted cache, and record-map
utilities — live under `internal/` and are not part of the public API.

This repository does not provide a snapshot HTTP endpoint. If an application
needs a remote ingestor service, that service and its route contract are caller
responsibilities.

## Runnable examples

The [`examples`](examples/) directory contains three end-to-end programs:

- [`cli`](examples/cli/) exports `index.html`, assets, CSS, and the sample
  interaction script to a directory.
- [`server`](examples/server/) fetches and renders pages on demand and keeps
  downloaded assets in memory.
- [`fullapp`](examples/fullapp/) is a copyable starter with on-disk assets,
  internal page navigation, styling, interactions, and KaTeX support.

```sh
go run ./examples/cli -url "https://www.notion.so/<public-page>" -out ./out
go run ./examples/server -addr :8080
go run ./examples/fullapp -addr :8080
```

See the [examples guide](examples/README.md) for flags, behavior, and production
hardening notes.

## Supported blocks

The renderer covers the common public Notion block types: paragraphs and the
four heading levels; bulleted, numbered, and to-do lists; quotes, callouts, and
toggles; code (with a `language-*` class) and equations; tables and database
collection views; images, video, audio, files, PDFs, bookmarks, tweets, and the
common embed providers; columns, breadcrumbs, table of contents, synced blocks,
tabs, and subpage links. Unknown or unsupported block types degrade to a safe
default (a titled link when one is available) rather than failing the render.

## Stylesheet and JS

`RenderPage` emits semantic HTML and `notion-*` classes, but it does not inline
CSS or JavaScript. The canonical stylesheet for those classes ships embedded in
the `notion` package and is returned by `notion.StyleCSS()`, so it always
matches the renderer version you build against. Serve it (or write it to a
file) however your app prefers:

```go
http.HandleFunc("/notion.css", func(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	io.WriteString(w, notion.StyleCSS())
})
```

```html
<link rel="stylesheet" href="/notion.css">
<script defer src="/notion.js"></script>

<main
  class="notion-body-wrap"
  data-collection-lightbox-labels
  data-lightbox-previous="Previous"
  data-lightbox-next="Next"
  data-lightbox-close="Close"
  data-lightbox-label="Image viewer">
  <!-- RenderPage output goes here. -->
</main>
```

The stylesheet is self-contained: design tokens included, no external imports
or webfonts, and it works as a single file. It supports light and dark themes —
set `data-theme="dark"` (or `"light"`) on the document root to force one, or
leave the attribute off to follow the reader's `prefers-color-scheme`. To use
webfonts (for example Pretendard and JetBrains Mono), load them yourself and
override the `--font-sans` / `--font-mono` variables:

```css
@import url('https://cdn.jsdelivr.net/gh/orioncactus/pretendard@v1.3.9/dist/web/variable/pretendardvariable-dynamic-subset.min.css');
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:ital,wght@0,400;0,500;0,600;1,400&display=swap');

:root {
  --font-sans: 'Pretendard Variable', Pretendard, system-ui, sans-serif;
  --font-mono: 'JetBrains Mono', ui-monospace, monospace;
}
```

A small sample interaction script, [`examples/notion.js`](examples/notion.js),
covers the renderer's `data-*` hooks: Notion tabs, multi-view database tabs,
image lightboxes, code-copy buttons, and optional KaTeX hydration when
`window.katex` is present. The HTML and stylesheet work without it. Treat the
script as a starting point and adapt interactions to your own site.

`examples/fullapp` serves `notion.StyleCSS()` directly and embeds the sample
interaction script with `go:embed`, while loading KaTeX CSS/JS from a pinned
CDN URL.

### Code highlighting

Code blocks are emitted as plain, escaped `<pre><code class="language-…">`
elements; the renderer does not color syntax server-side. Apply your own CSS or
add a client-side highlighter such as [Prism.js](https://prismjs.com) to
colorize them. The embedded stylesheet already styles a Prism-compatible code
surface.

## Rendering Contract

The renderer returns static HTML and intentionally degrades unsafe or unsupported
surfaces. It does not execute provider HTML or scripts. Notion-hosted files
should be downloaded and served by the caller; public rendered HTML should use
caller-controlled asset URLs instead of direct signed Notion URLs.

### Page navigation

Page aliases, mentions, and database row titles become links only when the
caller supplies their destinations. `BuildPagePaths` discovers renderable page
records and referenced page IDs from a normalized record map:

```go
pagePaths, err := notion.BuildPagePaths(recordMap, snapshot.RootPageID)
if err != nil {
	return err
}

html, err := notion.RenderPage(notion.RenderInput{
	RecordMap:    recordMap,
	PageID:       snapshot.RootPageID,
	PagePaths:    pagePaths,
	ResourceSlug: "render", // emits /render/<page-id>
})
```

Applications that use query routes or external destinations can instead set
`RenderInput.PageURLs`. Explicit URLs take precedence over generated
`PagePaths` and are restricted to safe HTTP(S) or root-relative URLs. The
runnable server examples map discovered pages to `/render?id=<page-id>`; the
static CLI links them to their public Notion destinations.

### Asset handling

Notion-hosted file URLs are temporary. The official API documents
[file URLs](https://developers.notion.com/reference/file-object) as short-lived
URLs that should not be cached or statically referenced, and the unofficial web
API follows the same practical constraint. Treat every `AssetSnapshot.SignedURL`
as an immediate download URL, not as a URL to persist or expose in public HTML.

The intended asset flow is:

1. Call `ingest.FetchSnapshot` to obtain a `Snapshot`.
2. Iterate over `snapshot.Assets` and download each asset from
   `asset.SignedURL` when present. If signing failed, the snapshot remains
   usable but the asset may be unavailable until a later fetch.
3. Store the downloaded bytes in caller-controlled storage, such as a local
   static directory, object storage bucket, CDN, or authenticated media proxy.
4. Build `RenderInput.AssetURLs` from each asset key to the caller-controlled
   URL, then call `RenderPage`.
5. Before persistence, convert the result to the wire-compatible
   `notion.Snapshot`, validate it, and call `ScrubSnapshotForStorage` to remove
   ephemeral signed URLs from both `snapshot.Assets` and the record map.

`/notion-assets/` is only an example prefix used in tests and sample render
inputs. This library does not register an HTTP route or serve files from that
path. A production application must implement the storage and serving behavior
behind whatever public URL it places in `RenderInput.AssetURLs`.

`RenderInput.AssetURLs` accepts caller-controlled URLs keyed by the asset
identity. For robust rendering, map assets by `asset.BlockID`; for file
properties, rich-text asset links, and custom emoji, also map by `asset.Source`
or by the source URL without its query string.

```go
assetURLs := map[string]string{}
for _, asset := range snapshot.Assets {
	// Download asset.SignedURL now, store it yourself, and return a stable URL.
	publicURL, err := storeNotionAsset(ctx, asset)
	if err != nil {
		// Surface this through RenderInput.Warnings or your own logs.
		continue
	}
	assetURLs[asset.BlockID] = publicURL
	assetURLs[asset.Source] = publicURL
}

recordMap, err := json.Marshal(snapshot.Page.RecordMap)
if err != nil {
	return err
}

html, err := notion.RenderPage(notion.RenderInput{
	RecordMap: recordMap,
	PageID:    snapshot.RootPageID,
	AssetURLs: assetURLs,
})
```

`ingest.Snapshot` is the fetch-oriented type; `notion.Snapshot` is its
wire-compatible storage and rendering counterpart. Convert, validate, and
scrub a freshly fetched snapshot before persistence:

```go
snapshotJSON, err := json.Marshal(snapshot)
if err != nil {
	return err
}

var storedSnapshot notion.Snapshot
if err := json.Unmarshal(snapshotJSON, &storedSnapshot); err != nil {
	return err
}
if err := notion.ValidateSnapshot(&storedSnapshot, snapshot.RootPageID); err != nil {
	return err
}
safeSnapshot, err := notion.ScrubSnapshotForStorage(&storedSnapshot)
if err != nil {
	return err
}
// Persist safeSnapshot, which contains no signed Notion URLs.
```

When reading a `notion.Snapshot` back from caller-owned storage or transport,
validate it again against the requested page identity before rendering it. Its
record map is already a `json.RawMessage`, so it can be passed directly to
`RenderInput.RecordMap`.

If an asset URL is missing from `AssetURLs`, the renderer will not fall back to
the original Notion-hosted URL. It will omit the media or render an unavailable
file placeholder instead, so expired signed URLs and private Notion asset links
are not leaked into public output.

#### Unsafe signed URL rendering

For local debugging only, `RenderInput.UnsafeRenderNotionSignedURLs = true`
forces the renderer to emit Notion-hosted signed asset URLs from the record map
or original source when no `AssetURLs` entry exists.

```go
html, err := notion.RenderPage(notion.RenderInput{
	RecordMap: recordMap,
	PageID:    pageID,

	// Local debugging only. Never enable this in production.
	UnsafeRenderNotionSignedURLs: true,
})
```

Do not enable this option in production. It can leak private, short-lived asset
credentials into public HTML, produce pages that break after URL expiry, and
bypass the caller-controlled asset proxy/storage contract above.

## Production checklist

Before exposing rendered pages to users:

1. Keep finite fetch and render limits; the library defaults are a useful
   baseline.
2. Inspect `Snapshot.Errors` and `Snapshot.Truncated` so partial asset or
   collection results are visible to operators and readers.
3. Download signed asset URLs immediately, store the bytes on an origin you
   control, and pass only those stable URLs to `RenderInput.AssetURLs`.
4. Map linked pages through `RenderInput.PageURLs` or a route backed by
   `BuildPagePaths`.
5. Serve `notion.StyleCSS()` from the same module version as the renderer and
   copy only the interactions your application needs from `examples/notion.js`.
6. Convert fetched results to `notion.Snapshot`, validate them, and call
   `ScrubSnapshotForStorage` before persistence.
7. Add application-level timeouts, caching, rate limits, content-security
   policy, asset content-type controls, and logging.

## Troubleshooting

- **The page cannot be fetched:** verify that the exact URL opens in a private
  browser window without an account. Private or workspace-only pages are not
  available to the default high-level client.
- **Images or files are missing:** this is the safe default. Download each
  `AssetSnapshot.SignedURL` and populate `RenderInput.AssetURLs`; do not persist
  or publish signed Notion URLs.
- **Tabs, lightboxes, or copy buttons do nothing:** static HTML and CSS work
  without JavaScript, but those enhancements require the hooks demonstrated in
  [`examples/notion.js`](examples/notion.js).
- **Subpages or database rows are not navigable:** provide `PageURLs`, or use
  `BuildPagePaths` together with a matching application route.
- **An upgrade changes markup or behavior:** keep the module pinned, review the
  entries below, and compare the relevant version tags before upgrading.

## Changelog

This section summarizes user-visible changes. Follow the linked comparisons for
the complete code history.

### [v0.1.3](https://github.com/hwasub/unofficial-notion-go/tree/v0.1.3) — 2026-08-12

- Required HTTPS for private Notion API base URLs, except literal loopback
  addresses used by local tests.
- Prevented authentication cookies from following redirects and disabled
  ambient proxy inheritance for the default Notion HTTP client.
- Added bounded HTTP server settings and root-scoped asset reads to runnable
  examples, and restricted ephemeral cache directories to their owner.

[Compare v0.1.2...v0.1.3](https://github.com/hwasub/unofficial-notion-go/compare/v0.1.2...v0.1.3)

### [v0.1.2](https://github.com/hwasub/unofficial-notion-go/tree/v0.1.2) — 2026-07-25

- Expanded internal page navigation to aliases, mentions, rich-text references,
  database rows, and explicit image hyperlinks.
- Improved nested collection hydration and collection presentation, including
  property wrapping, normalized titles, URL labels, grouping, and formulas.
- Made tab content progressively available without JavaScript; the sample
  script now enhances it into an interactive tab interface.
- Fixed heading-anchor and table-of-contents traversal edge cases and hardened
  malformed or cyclic record-map handling.
- Updated all runnable examples to match the navigation, asset, styling, and
  interaction contracts.

[Compare v0.1.1...v0.1.2](https://github.com/hwasub/unofficial-notion-go/compare/v0.1.1...v0.1.2)

### [v0.1.1](https://github.com/hwasub/unofficial-notion-go/tree/v0.1.1) — 2026-07-10

- Added snapshot schema and page-identity validation for stored or transported
  snapshots.
- Prevented incomplete block snapshots and surfaced bounded collection-repair
  failures through structured errors and truncation flags.
- Hardened upstream response validation, cancellation, collection fetching, and
  render-cache limit isolation.
- Kept the `ingest` and `notion` snapshot wire formats explicitly compatible.

[Compare v0.1.0...v0.1.1](https://github.com/hwasub/unofficial-notion-go/compare/v0.1.0...v0.1.1)

### [v0.1.0](https://github.com/hwasub/unofficial-notion-go/tree/v0.1.0) — 2026-06-25

- Added public render input and output limits.
- Rendered the root page title with a stable heading contract.
- Reported collection-repair truncation and other non-fatal ingest problems.
- Hardened render caching, JSON response handling, and example asset budgets.

Earlier pre-`v0.1.0` releases are available on the
[tags page](https://github.com/hwasub/unofficial-notion-go/tags).

## License

MIT
