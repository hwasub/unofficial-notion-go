package notion

import "encoding/json"

// Snapshot is a point-in-time capture of a Notion page and its assets. It
// carries everything the renderer needs plus metadata about how the capture was
// bounded and whether it was truncated.
//
// It is the wire-compatible client-side mirror of ingest.Snapshot: the fields
// and JSON shape match, except RecordMap here is a raw json.RawMessage rather
// than a parsed ingest.NormalizedRecordMap. Use this type when decoding stored
// or transported snapshot JSON; use ingest.Snapshot when fetching directly.
type Snapshot struct {
	SchemaVersion int             `json:"schema_version"`
	FetchedAt     string          `json:"fetched_at"`   // RFC 3339 time the snapshot was captured
	RootPageID    string          `json:"root_page_id"` // ID of the page the snapshot was rooted at
	SourceURL     string          `json:"source_url"`   // original Notion URL the snapshot was fetched from
	Limits        SnapshotLimits  `json:"limits"`
	Page          PageSnapshot    `json:"page"`
	Assets        []AssetSnapshot `json:"assets"`
	Errors        []SnapshotError `json:"errors"`    // non-fatal fetch errors encountered while capturing
	Truncated     SnapshotFlags   `json:"truncated"` // which limits were hit during capture
}

// SnapshotLimits records the asset and block caps applied while capturing a
// Snapshot.
type SnapshotLimits struct {
	MaxAssets int `json:"max_assets"`
	MaxBlocks int `json:"max_blocks"`
}

// SnapshotFlags reports whether limits caused content to be dropped, indicating
// the Snapshot may be missing assets or blocks.
type SnapshotFlags struct {
	Assets bool `json:"assets"`
	Blocks bool `json:"blocks"`
}

// SnapshotError describes a non-fatal error that occurred while fetching a
// particular page during capture.
type SnapshotError struct {
	PageID  string `json:"page_id"`
	Message string `json:"message"`
}

// These constants are the recognized RenderWarning.Kind values, identifying the
// class of degraded content the renderer surfaces to readers.
const (
	RenderWarningFetchErrors         = "fetch_errors"          // one or more pages failed to fetch
	RenderWarningAssetDownloadErrors = "asset_download_errors" // one or more assets failed to download
	RenderWarningAssetsTruncated     = "assets_truncated"      // the asset limit was reached
	RenderWarningBlocksTruncated     = "blocks_truncated"      // the block limit was reached
)

// RenderWarning is a single advisory shown to readers when part of a page could
// not be captured or rendered. Kind is one of the RenderWarning* constants and
// Count is the number of affected items (or 1 for boolean conditions).
type RenderWarning struct {
	Kind  string
	Count int
}

// RenderWarningsForSnapshot derives the RenderWarning list implied by a
// Snapshot's errors and truncation flags. It returns nil for a nil snapshot.
func RenderWarningsForSnapshot(snapshot *Snapshot) []RenderWarning {
	if snapshot == nil {
		return nil
	}
	warnings := make([]RenderWarning, 0, 4)
	if len(snapshot.Errors) > 0 {
		warnings = append(warnings, RenderWarning{Kind: RenderWarningFetchErrors, Count: len(snapshot.Errors)})
	}
	if snapshot.Truncated.Assets {
		warnings = append(warnings, RenderWarning{Kind: RenderWarningAssetsTruncated, Count: 1})
	}
	if snapshot.Truncated.Blocks {
		warnings = append(warnings, RenderWarning{Kind: RenderWarningBlocksTruncated, Count: 1})
	}
	return warnings
}

// PageSnapshot holds a single captured page: its identity, title, and the raw
// Notion record map the renderer consumes.
type PageSnapshot struct {
	PageID     string          `json:"page_id"`
	Title      string          `json:"title"`
	RecordMap  json.RawMessage `json:"record_map"`  // raw Notion record map for this page
	AssetCount int             `json:"asset_count"` // number of assets referenced by the page
}

// AssetSnapshot describes one media or file asset referenced by a page,
// including its render key, origin, and any temporary signed download URL.
type AssetSnapshot struct {
	BlockID   string `json:"block_id"`   // asset key, usually a block ID with an optional role suffix
	PageID    string `json:"page_id"`    // page the referencing block belongs to
	Type      string `json:"type"`       // asset kind (image, video, file, ...)
	Source    string `json:"source"`     // original Notion source URL
	SignedURL string `json:"signed_url"` // temporary signed URL; scrubbed before storage
	Filename  string `json:"filename"`
}
