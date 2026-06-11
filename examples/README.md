# Examples

Runnable programs that show the full library flow end to end: fetch a public
Notion page, download its assets, and render sanitized HTML.

Both reach out to Notion's internal web API, so they need network access and a
**public** Notion page URL. They are demonstrations, not production code.

## `cli` — render a page to a file

Renders a page to a self-contained directory (`index.html` + `assets/` +
`notion.css` + `notion.js`).

```sh
go run ./examples/cli \
  -url "https://www.notion.so/Example-1ad6e61cf82480c9a6c4d251043457d3" \
  -out ./out
```

Open `out/index.html` in a browser. Flags: `-url` (required), `-out` (default
`out`), `-css` (stylesheet to copy alongside the HTML; defaults to the embedded
`notion.StyleCSS()`, pass `none` to skip), `-js` (script to copy alongside the
HTML; default `examples/notion.js`), `-max-assets`.

## `server` — render pages over HTTP

Starts a small HTTP server that fetches, renders, and serves pages on demand,
proxying their assets from memory under `/assets/` — the caller-controlled
asset-serving pattern the library expects. It serves only detected image, audio,
and video bytes inline; other files are sent as downloads.

```sh
go run ./examples/server -addr :8080 -js examples/notion.js
```

Open <http://localhost:8080/> and paste a public Notion page URL. The
stylesheet served at `/notion.css` defaults to the embedded `notion.StyleCSS()`;
pass `-css <path>` to serve a custom file, or `-css none` to skip it.

## `fullapp` — starter app

Starts a fuller HTTP app intended as a copyable starting point for real users.
It serves the renderer stylesheet from `notion.StyleCSS()`, embeds its own app
chrome CSS and the sample interaction JS with `go:embed`, loads KaTeX CSS/JS
from a pinned CDN URL for equation hydration, and stores downloaded Notion
assets on disk before re-serving them under `/assets/`.

```sh
go run ./examples/fullapp -addr :8080
```

Open <http://localhost:8080/> and paste a public Notion page URL. Flags:
`-addr`, `-asset-dir` (default: a temporary directory), and `-max-assets`.

## Notes

- **Signed asset URLs are short-lived.** Each `AssetSnapshot.SignedURL` must be
  downloaded immediately and re-served from your own storage; never put a signed
  Notion URL in public HTML. The runnable examples download assets and rewrite
  `RenderInput.AssetURLs` to local URLs accordingly.
- **Asset serving still needs production hardening.** The server demo keeps
  assets in memory and sends non-media bytes as attachments so HTML/SVG files do
  not execute on the app origin. The full app stores assets on disk but still
  uses the same conservative content-type policy. Production deployments should
  use a dedicated asset origin, explicit content-type policy, CSP, caching, and
  eviction.
- **Persisting snapshots?** Call `notion.ScrubSnapshotForStorage` first to strip
  ephemeral signed URLs before writing a snapshot to durable storage. The
  examples render immediately, so they skip this step.
- **Styling.** The canonical stylesheet for the `notion-*` classes the renderer
  emits ships inside the library: `notion.StyleCSS()`. It is self-contained
  (system fonts, no external imports); load webfonts yourself and override
  `--font-sans` / `--font-mono` if you want them. All three examples use it by
  default.
- **Interactions.** [`notion.js`](notion.js) is a sample script for the
  renderer's `data-*` hooks. It enables tabs, multi-view database tabs,
  image lightboxes, code-copy buttons, and optional KaTeX hydration.
- **Code highlighting** is left to the client: code blocks carry
  `class="language-…"` for a highlighter such as Prism.js or your own CSS.
