// Package notion is a safe static HTML renderer for public Notion page
// snapshots.
//
// The renderer turns a Notion record map into sanitized, self-contained HTML.
// RenderPage is the entry point and takes a RenderInput describing the page, its
// asset URLs, link paths, and render warnings; the emitted HTML escapes
// untrusted text and only references URLs that pass the package's safety checks.
//
// The actual fetching from Notion and normalization of its data into a Snapshot
// live in the companion ingest package. Applications are responsible for their
// own HTTP serving, storage, and transport when snapshots cross process
// boundaries.
package notion
