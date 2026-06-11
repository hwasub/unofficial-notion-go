package notion

import _ "embed"

//go:embed style.css
var styleCSS string

// StyleCSS returns the canonical stylesheet for the HTML emitted by
// RenderPage. It is self-contained — design tokens included, no external
// imports or webfonts — so it can be served or inlined as-is.
//
// Dark mode: set data-theme="dark" (or "light") on the document root to force
// a theme; without a data-theme attribute the stylesheet follows the reader's
// prefers-color-scheme. Interactive niceties (tabs, lightbox, code copy)
// additionally need a script such as examples/notion.js; the stylesheet works
// without it.
func StyleCSS() string { return styleCSS }
