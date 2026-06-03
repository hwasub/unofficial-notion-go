// Package notionapi is a low-level client for Notion's private (unofficial) web
// API. It authenticates with a token_v2 cookie and speaks the loadPageChunk,
// queryCollection, syncRecordValuesMain, and getSignedFileUrls endpoints that
// back www.notion.so, returning the raw record maps Notion's own frontend
// consumes.
//
// Construct a Client with New and tune it through the With* options
// (WithAuthToken, WithActiveUser, WithUserTimeZone, WithHTTPClient,
// WithAPIBaseURL, WithMaxResponseBytes). The zero value is usable with defaults,
// but New is preferred. A Client is safe for concurrent use.
//
// GetPage is the high-level entry point: it drives the lower-level calls to
// assemble a complete record map for a page, fetching missing blocks, resolving
// collection data, and signing file URLs as needed. The lower-level methods —
// GetPageRaw, GetCollectionData, GetBlocks, GetSignedFileURLs, AddSignedURLs,
// and the generic Fetch — are exposed for callers that need direct control over
// individual endpoints.
//
// Failed requests return an *HTTPError carrying the upstream status code, an
// error code (such as ErrorCodeMaxResponseBytesExceeded when a response exceeds
// the configured byte cap), and a Retry-After hint; RetryAfterToMS parses that
// hint into a backoff duration.
//
// This package is the transport layer the rest of the module builds on. Most
// callers that just want a renderable page snapshot should use package ingest,
// which wraps a Client with normalization, repair, asset discovery, and limits;
// reach for notionapi directly when you need raw endpoint access or want to
// supply a custom client to ingest.FetchOptions.
//
// This is not Notion's official REST API at api.notion.com. It calls internal
// endpoints under www.notion.so/api/v3, which are undocumented and may change or
// stop working without notice. Review Notion's terms before relying on it.
package notionapi
