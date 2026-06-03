package ingest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/hwasub/unofficial-notion-go/internal/notionid"
	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
	"github.com/hwasub/unofficial-notion-go/notionapi"
)

// PageClient is the subset of a Notion client that FetchSnapshot depends on. It
// fetches a page's record map, signs asset URLs, and queries collection data
// used to repair incomplete collection views. notionapi.Client satisfies it,
// and FetchSnapshot constructs a default one when none is supplied.
type PageClient interface {
	// GetPage fetches the record map for the page identified by pageID.
	GetPage(ctx context.Context, pageID string, opts notionapi.PageOptions) (map[string]any, error)
	// AddSignedURLs populates signed URLs for the given content block IDs in
	// the record map. It is best-effort and may fail without aborting a fetch.
	AddSignedURLs(ctx context.Context, recordMap map[string]any, contentBlockIDs []string) error
	// GetCollectionData queries a collection view, used to repair collection
	// query results that are missing or incomplete in the page record map.
	GetCollectionData(ctx context.Context, collectionID string, collectionViewID string, collectionView any, opts notionapi.CollectionOptions) (map[string]any, error)
}

// FetchOptions configures a FetchSnapshot call. All fields are optional: a nil
// Client causes a default notionapi client to be constructed, a nil Now uses
// time.Now, and a nil Log discards events.
type FetchOptions struct {
	Client PageClient                                // Notion client to use; defaults to a new notionapi client
	Now    func() time.Time                          // clock used for timestamps and durations; defaults to time.Now
	Log    func(event string, fields map[string]any) // structured event sink; defaults to a no-op
}

// FetchSnapshot fetches a single public Notion page identified by request,
// repairs incomplete collection queries, normalizes the record map, and
// discovers its assets, returning a Snapshot bounded by limits. Asset URL
// signing is best-effort and reported via Snapshot.Errors rather than failing
// the call. Validation and upstream failures are returned as an *HTTPError.
func FetchSnapshot(ctx context.Context, request FetchRequest, limits RequestLimits, opts FetchOptions) (*Snapshot, error) {
	rootPageID := notionid.ParsePageID(firstNonEmpty(request.PageID, request.URL))
	if rootPageID == "" {
		return nil, &HTTPError{StatusCode: 400, Message: "valid Notion page_id or URL is required"}
	}
	limits = normalizeRequestLimits(limits)
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	client := opts.Client
	if client == nil {
		// WithMaxResponseBytes treats a non-positive value as "use the default".
		client = notionapi.New(notionapi.WithMaxResponseBytes(int64(limits.MaxResponseBytes)))
	}
	log := opts.Log
	if log == nil {
		log = func(string, map[string]any) {}
	}

	startedAt := now()
	fetchedAt := now().UTC().Format(time.RFC3339Nano)
	log("notion_snapshot_started", map[string]any{
		"root_page_id": rootPageID,
		"max_assets":   limits.MaxAssets,
		"max_blocks":   limits.MaxBlocks,
		"timeout_ms":   limits.TimeoutMS,
	})

	pageCtx, cancel := context.WithTimeout(ctx, time.Duration(limits.TimeoutMS)*time.Millisecond)
	defer cancel()

	pageStartedAt := now()
	log("notion_page_fetch_started", map[string]any{"page_id": rootPageID, "depth": 0})
	recordMap, err := client.GetPage(pageCtx, rootPageID, notionapi.PageOptions{
		Concurrency:             2,
		FetchMissingBlocks:      true,
		FetchCollections:        true,
		FetchRelationPages:      false,
		SignFileURLs:            false,
		ChunkLimit:              100,
		CollectionReducerLimit:  999,
		ThrowOnCollectionErrors: false,
		MaxBlocks:               limits.MaxBlocks,
	})
	if err == nil {
		err = rejectRecordMapBlockLimit(recordMap, limits.MaxBlocks)
	}
	if err == nil {
		err = repairCollectionQueries(pageCtx, client, recordMap, limits.MaxBlocks, log)
	}
	if err == nil {
		err = rejectRecordMapBlockLimit(recordMap, limits.MaxBlocks)
	}
	if err != nil {
		mapped := mapNotionError(err)
		log("notion_page_fetch_failed", map[string]any{
			"page_id":        rootPageID,
			"depth":          0,
			"status_code":    mapped.StatusCode,
			"error":          mapped.Code,
			"message":        mapped.Message,
			"retry_after_ms": mapped.RetryAfterMS,
		})
		return nil, mapped
	}

	// Asset URL signing is best-effort: a failure here does not abort the
	// fetch. We log it and surface it as a SnapshotError so the renderer can
	// emit a warning, while still returning a usable snapshot.
	snapshotErrors := []SnapshotError{}
	if err := client.AddSignedURLs(pageCtx, recordMap, recordMapBlockIDs(recordMap)); err != nil {
		log("notion_asset_signing_failed", map[string]any{
			"page_id": rootPageID,
			"error":   err.Error(),
		})
		snapshotErrors = append(snapshotErrors, SnapshotError{
			PageID:  rootPageID,
			Message: "asset URL signing failed: " + err.Error(),
		})
	}

	blockCount := recordMapBlockCount(recordMap)
	normalized := NormalizeRecordMap(recordMap)
	rootBlock := normalized.Block[rootPageID]
	if rootBlock == nil {
		rootBlock = normalized.Block[notionid.CompactID(rootPageID)]
	}

	pageAssets := CollectAssets(normalized, rootPageID)
	assets := make([]AssetSnapshot, 0, min(len(pageAssets), limits.MaxAssets))
	for _, asset := range pageAssets {
		if len(assets) >= limits.MaxAssets {
			break
		}
		assets = append(assets, asset)
	}
	page := PageSnapshot{
		PageID:     rootPageID,
		Title:      GetTitle(rootBlock),
		RecordMap:  normalized,
		AssetCount: len(pageAssets),
	}

	log("notion_page_fetch_completed", map[string]any{
		"page_id":      rootPageID,
		"depth":        0,
		"blocks":       blockCount,
		"assets":       len(pageAssets),
		"total_assets": len(assets),
		"total_blocks": blockCount,
		"duration_ms":  int(now().Sub(pageStartedAt) / time.Millisecond),
	})

	truncatedAssets := len(pageAssets) > len(assets)
	truncatedBlocks := false
	log("notion_snapshot_completed", map[string]any{
		"root_page_id":     rootPageID,
		"assets":           len(assets),
		"errors":           len(snapshotErrors),
		"total_blocks":     blockCount,
		"truncated_assets": truncatedAssets,
		"truncated_blocks": truncatedBlocks,
		"duration_ms":      int(now().Sub(startedAt) / time.Millisecond),
	})

	return &Snapshot{
		SchemaVersion: 1,
		FetchedAt:     fetchedAt,
		RootPageID:    rootPageID,
		SourceURL:     request.URL,
		Limits: SnapshotLimits{
			MaxAssets: limits.MaxAssets,
			MaxBlocks: limits.MaxBlocks,
		},
		Page:   page,
		Assets: assets,
		Errors: snapshotErrors,
		Truncated: SnapshotFlags{
			Assets: truncatedAssets,
			Blocks: truncatedBlocks,
		},
	}, nil
}

func mapNotionError(cause error) *HTTPError {
	if cause == nil {
		return nil
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return NewHTTPError("Notion page fetch timed out", 504, "notion_fetch_timeout")
	}
	if httpErr, ok := AsHTTPError(cause); ok {
		if httpErr.StatusCode == 429 {
			httpErr.Code = "notion_rate_limited"
		}
		return httpErr
	}
	var upstream *notionapi.HTTPError
	if errors.As(cause, &upstream) {
		if upstream.Code == notionapi.ErrorCodeMaxResponseBytesExceeded || upstream.Code == notionapi.ErrorCodeMaxBlocksExceeded {
			return &HTTPError{
				StatusCode:   http.StatusRequestEntityTooLarge,
				Code:         upstream.Code,
				Message:      upstream.Message,
				RetryAfterMS: upstream.RetryAfterMS,
			}
		}
		switch upstream.StatusCode {
		case 401, 403:
			return &HTTPError{StatusCode: upstream.StatusCode, Code: "notion_upstream_error", Message: upstream.Message, RetryAfterMS: upstream.RetryAfterMS}
		case 404:
			return &HTTPError{StatusCode: 404, Code: "notion_page_not_found", Message: upstream.Message, RetryAfterMS: upstream.RetryAfterMS}
		case 429:
			return &HTTPError{StatusCode: 429, Code: "notion_rate_limited", Message: upstream.Message, RetryAfterMS: upstream.RetryAfterMS}
		default:
			if upstream.StatusCode >= 500 {
				return &HTTPError{StatusCode: 502, Code: "notion_upstream_error", Message: upstream.Message, RetryAfterMS: upstream.RetryAfterMS}
			}
		}
	}
	if strings.Contains(strings.ToLower(cause.Error()), "notion page not found") {
		return NewHTTPError(cause.Error(), 404, "notion_page_not_found")
	}
	return &HTTPError{StatusCode: 500, Code: "internal_error", Message: cause.Error()}
}

func recordMapBlockCount(recordMap map[string]any) int {
	return len(notionrecordmap.AsMap(recordMap["block"]))
}

func recordMapBlockIDs(recordMap map[string]any) []string {
	blocks := notionrecordmap.AsMap(recordMap["block"])
	ids := make([]string, 0, len(blocks))
	for id := range blocks {
		ids = append(ids, id)
	}
	return ids
}

func rejectRecordMapBlockLimit(recordMap map[string]any, maxBlocks int) error {
	if maxBlocks > 0 && recordMapBlockCount(recordMap) > maxBlocks {
		return NewHTTPError("max block limit exceeded", http.StatusRequestEntityTooLarge, "max_blocks_exceeded")
	}
	return nil
}
