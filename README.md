# unofficial-notion-go

[![Go Reference](https://pkg.go.dev/badge/github.com/hwasub/unofficial-notion-go.svg)](https://pkg.go.dev/github.com/hwasub/unofficial-notion-go)
[![CI](https://github.com/hwasub/unofficial-notion-go/actions/workflows/ci.yml/badge.svg)](https://github.com/hwasub/unofficial-notion-go/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/hwasub/unofficial-notion-go)](https://goreportcard.com/report/github.com/hwasub/unofficial-notion-go)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Unofficial Go client and renderer for public Notion pages.

It has zero third-party dependencies — only the Go standard library.

This library is intended to provide a Go-native public Notion page workflow:

- fetch a public Notion page through Notion's internal web API;
- normalize the returned record map into a stable snapshot shape;
- discover Notion-hosted assets that callers may download and proxy;
- render sanitized, static HTML for public readers.

## Install

```sh
go get github.com/hwasub/unofficial-notion-go
```

Requires Go 1.25 or newer.

## Status

Experimental and **pre-1.0**: the public API may change between minor versions
until a v1.0.0 release. Pin a version before upgrading.

This project is not affiliated with Notion and does not use Notion's official
REST API at `api.notion.com`. It calls internal endpoints under
`www.notion.so/api/v3`, which may change or stop working without notice.

Review Notion's current terms and policies before using this library in a
product. Callers are responsible for permissions, rate limiting, data handling,
and compliance with Notion's terms.

## Packages

The module exposes three public packages, layered low to high:

- `notionapi`: low-level client for Notion's private web API — authenticate,
  fetch a page's record map, query collections, fetch blocks, and sign file
  URLs. Use it directly when you want raw endpoint access.
- `ingest`: high-level page snapshot built on `notionapi` — snapshot fetch,
  normalization, collection repair, asset discovery, limits, and errors.
- `notion`: safe static HTML renderer plus snapshot and render-warning types.

Internal helpers — ID parsing, the byte-budgeted cache, and record-map
utilities — live under `internal/` and are not part of the public API.

This repository does not provide a snapshot HTTP endpoint. If an application
needs a remote ingestor service, that service and its route contract are caller
responsibilities.

## Supported blocks

The renderer covers the common public Notion block types: paragraphs and the
four heading levels; bulleted, numbered, and to-do lists; quotes, callouts, and
toggles; code (with a `language-*` class) and equations; tables and database
collection views; images, video, audio, files, PDFs, bookmarks, tweets, and the
common embed providers; columns, breadcrumbs, table of contents, synced blocks,
tabs, and subpage links. Unknown or unsupported block types degrade to a safe
default (a titled link when one is available) rather than failing the render.

## Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/hwasub/unofficial-notion-go/ingest"
)

func main() {
	defaults := ingest.DefaultLimitsFromEnv()
	req := ingest.FetchRequest{
		URL:       "https://www.notion.so/example/Example-1ad6e61cf82480c9a6c4d251043457d3",
		MaxAssets: 200,
	}
	snapshot, err := ingest.FetchSnapshot(
		context.Background(),
		req,
		ingest.RequestLimitsForRequest(req, defaults),
		ingest.FetchOptions{},
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(snapshot.RootPageID)
}
```

For complete, runnable programs that fetch a page, download its assets, and
render HTML — a CLI, a small HTTP server, and a starter app — see the
[`examples`](examples/) directory:

```sh
go run ./examples/cli -url "https://www.notion.so/<public-page>" -out ./out
go run ./examples/server -addr :8080
go run ./examples/fullapp -addr :8080
```

## Sample CSS and JS

`RenderPage` emits semantic HTML and `notion-*` classes, but it does not inline
CSS or JavaScript. A standalone sample stylesheet and small interaction script
are included as starting points for the classes and data attributes the renderer
emits:

- [`examples/notion.css`](examples/notion.css)
- [`examples/notion.js`](examples/notion.js)

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

The sample CSS includes the design tokens it needs, so it can be copied or
served as a single file for quick starts. It also `@import`s the Pretendard and
JetBrains Mono web fonts from public CDNs; self-host or drop those imports if you
would rather not depend on third-party font hosts. The sample JS enables Notion
tabs, multi-view database tabs, image lightboxes, code-copy buttons, and optional
KaTeX hydration when `window.katex` is present. Treat both files as starting
points and adjust typography, colors, layout, and interactions to your own site.

`examples/fullapp` embeds the local sample stylesheet and interaction script
with `go:embed`, while loading KaTeX CSS/JS from a pinned CDN URL.

### Code highlighting

Code blocks are emitted as plain, escaped `<pre><code class="language-…">`
elements; the renderer does not color syntax server-side. Apply your own CSS or
add a client-side highlighter such as [Prism.js](https://prismjs.com) to
colorize them. The sample stylesheet already styles a Prism-compatible code
surface.

## Rendering Contract

The renderer returns static HTML and intentionally degrades unsafe or unsupported
surfaces. It does not execute provider HTML or scripts. Notion-hosted files
should be downloaded and served by the caller; public rendered HTML should use
caller-controlled asset URLs instead of direct signed Notion URLs.

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
5. Before persisting snapshots, call `ScrubSnapshotForStorage` to remove
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

When rendering a `notion.Snapshot` decoded from caller-owned storage or
transport, the record map is already a `json.RawMessage`, so it can be passed
directly to `RenderInput.RecordMap`.

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

## License

MIT
