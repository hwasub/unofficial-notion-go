package ingest

// FetchRequest identifies the Notion page to snapshot and carries optional
// per-request limit overrides. Either PageID or URL must resolve to a valid
// page ID. A zero value for a limit field means "use the default".
type FetchRequest struct {
	PageID    string `json:"page_id,omitempty"`
	URL       string `json:"url,omitempty"`
	MaxAssets int    `json:"max_assets,omitempty"`
	MaxBlocks int    `json:"max_blocks,omitempty"`
	TimeoutMS int    `json:"timeout_ms,omitempty"`
}

// DefaultLimits holds the server-wide ingest limits and the hard ceilings that
// per-request overrides are clamped to. Construct one with DefaultLimitsFromEnv.
type DefaultLimits struct {
	HardMaxAssets    int // upper bound a request may raise MaxAssets to
	HardMaxBlocks    int // upper bound a request may raise MaxBlocks to
	MaxAssets        int // default maximum assets returned per snapshot
	MaxBlocks        int // default maximum blocks allowed before failing
	TimeoutMS        int // default page-fetch timeout in milliseconds
	MaxResponseBytes int // maximum accepted Notion response size in bytes
}

// RequestLimits holds the effective limits applied to a single FetchSnapshot
// call after defaults and per-request overrides have been resolved and clamped.
type RequestLimits struct {
	MaxAssets        int // maximum assets returned in the snapshot
	MaxBlocks        int // maximum blocks allowed before the fetch fails
	TimeoutMS        int // page-fetch timeout in milliseconds
	MaxResponseBytes int // maximum accepted Notion response size in bytes
}

// Snapshot is the self-contained result of fetching and normalizing a single
// public Notion page, including its normalized record map, discovered assets,
// any non-fatal errors, and flags indicating whether limits truncated output.
//
// The notion package exposes a wire-compatible mirror, notion.Snapshot, for
// callers that decode stored or transported snapshot JSON and only render; that
// type carries RecordMap as a raw json.RawMessage instead of a
// NormalizedRecordMap.
type Snapshot struct {
	SchemaVersion int             `json:"schema_version"` // snapshot format version
	FetchedAt     string          `json:"fetched_at"`     // RFC3339Nano UTC timestamp of the fetch
	RootPageID    string          `json:"root_page_id"`   // normalized ID of the fetched page
	SourceURL     string          `json:"source_url"`     // original request URL, if any
	Limits        SnapshotLimits  `json:"limits"`         // limits in effect for this snapshot
	Page          PageSnapshot    `json:"page"`           // the fetched page and its record map
	Assets        []AssetSnapshot `json:"assets"`         // discovered assets, capped at MaxAssets
	Errors        []SnapshotError `json:"errors"`         // non-fatal errors encountered during the fetch
	Truncated     SnapshotFlags   `json:"truncated"`      // whether assets or blocks were truncated
}

// SnapshotLimits records the asset and block limits that applied when a
// Snapshot was produced.
type SnapshotLimits struct {
	MaxAssets int `json:"max_assets"`
	MaxBlocks int `json:"max_blocks"`
}

// SnapshotFlags reports whether output was truncated because a limit dropped
// content. FetchSnapshot drops assets beyond MaxAssets, but it fails rather than
// returning a partial block set when MaxBlocks is exceeded.
type SnapshotFlags struct {
	Assets bool `json:"assets"`
	Blocks bool `json:"blocks"`
}

// SnapshotError describes a non-fatal problem encountered while building a
// Snapshot, such as a failure to sign asset URLs. It identifies the affected
// page and carries a human-readable message.
type SnapshotError struct {
	PageID  string `json:"page_id"`
	Message string `json:"message"`
}

// PageSnapshot is the fetched page itself: its ID, plain-text title, the
// normalized record map of its blocks, and the total number of assets
// discovered on the page (which may exceed the number returned in the Snapshot
// when limits truncate the list).
type PageSnapshot struct {
	PageID     string              `json:"page_id"`
	Title      string              `json:"title"`
	RecordMap  NormalizedRecordMap `json:"record_map"`
	AssetCount int                 `json:"asset_count"`
}

// AssetSnapshot describes a single Notion-hosted asset discovered on a page,
// such as an image, file, page cover or icon, or custom emoji.
type AssetSnapshot struct {
	BlockID   string `json:"block_id"`   // ID of the owning block, with a role suffix for icons/covers
	PageID    string `json:"page_id"`    // normalized ID of the page the asset belongs to
	Type      string `json:"type"`       // block type or "custom_emoji"
	Source    string `json:"source"`     // raw Notion-hosted source URL
	SignedURL string `json:"signed_url"` // signed or proxied URL, falling back to Source
	Filename  string `json:"filename"`   // file name derived from the asset URL
}

// NormalizedRecordMap is the scrubbed, stably-keyed form of a Notion record
// map. Records are keyed by normalized block ID, sensitive and volatile fields
// are removed (see Scrub), and collection queries are repaired in place. It is
// the canonical record map carried inside a PageSnapshot.
type NormalizedRecordMap struct {
	Block            map[string]map[string]any `json:"block"`             // blocks keyed by normalized ID
	Collection       map[string]map[string]any `json:"collection"`        // collections keyed by normalized ID
	CollectionView   map[string]map[string]any `json:"collection_view"`   // collection views keyed by normalized ID
	CollectionQuery  any                       `json:"collection_query"`  // repaired query results, keyed by collection then view ID
	Automation       map[string]map[string]any `json:"automation"`        // automations keyed by normalized ID
	AutomationAction map[string]map[string]any `json:"automation_action"` // automation actions keyed by normalized ID
	CustomEmojis     map[string]string         `json:"custom_emojis"`     // custom emoji URLs keyed by emoji ID
	SignedURLs       map[string]string         `json:"signed_urls"`       // signed asset URLs keyed by block or source key
}
