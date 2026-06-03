// Package ingest takes a snapshot of a single public Notion page. It fetches
// the page's record map through a PageClient, repairs incomplete collection
// queries, normalizes and scrubs the raw record map into a stable
// NormalizedRecordMap, and discovers the page's Notion-hosted assets. The
// result is a self-contained Snapshot suitable for caching or rendering.
//
// Behaviour is bounded by RequestLimits (asset/block ceilings, timeout, and a
// response-size cap), which are derived from DefaultLimits plus per-request
// overrides. Limits and defaults can be seeded from the environment via
// DefaultLimitsFromEnv. Upstream and validation failures are surfaced as an
// HTTPError carrying an HTTP status code and a stable machine-readable code.
package ingest
