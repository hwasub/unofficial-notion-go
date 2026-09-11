package notion

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/hwasub/unofficial-notion-go/internal/notionasset"
)

// ScrubSnapshotForStorage returns a copy of snapshot safe to persist: it
// normalizes all page, root, and asset IDs and strips ephemeral signed URLs
// (both per-asset SignedURL fields and the record map's signed_urls entry) so
// expiring credentials are never stored. It returns nil for a nil snapshot and
// leaves the input unmodified.
func ScrubSnapshotForStorage(snapshot *Snapshot) (*Snapshot, error) {
	if snapshot == nil {
		return nil, nil
	}
	out := *snapshot
	page := snapshot.Page
	recordMap, err := scrubRecordMapSignedURLs(page.RecordMap)
	if err != nil {
		return nil, err
	}
	page.RecordMap = recordMap
	page.PageID = NormalizeID(page.PageID)
	out.Page = page
	out.Assets = make([]AssetSnapshot, len(snapshot.Assets))
	for i, asset := range snapshot.Assets {
		clean := asset
		clean.BlockID = normalizeAssetKey(clean.BlockID)
		clean.PageID = NormalizeID(clean.PageID)
		clean.SignedURL = ""
		clean.Source = notionasset.ForStorage(clean.Source)
		out.Assets[i] = clean
	}
	out.RootPageID = NormalizeID(out.RootPageID)
	return &out, nil
}

func scrubRecordMapSignedURLs(recordMap json.RawMessage) (json.RawMessage, error) {
	if len(recordMap) == 0 {
		return recordMap, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(recordMap, &raw); err != nil {
		return nil, err
	}
	delete(raw, "signed_urls")
	clean, err := scrubAssetValues(raw, 0)
	if err != nil {
		return nil, err
	}
	return json.Marshal(clean)
}

func normalizeAssetKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	base, suffix, ok := strings.Cut(key, ":")
	base = NormalizeID(base)
	if !ok {
		return base
	}
	return base + ":" + suffix
}

func scrubAssetValues(value any, depth int) (any, error) {
	if depth > 128 {
		return nil, errors.New("snapshot exceeds storage nesting limit")
	}
	switch v := value.(type) {
	case string:
		return notionasset.ForStorage(v), nil
	case map[string]any:
		for k, child := range v {
			clean, err := scrubAssetValues(child, depth+1)
			if err != nil {
				return nil, err
			}
			v[k] = clean
		}
	case []any:
		for i, child := range v {
			clean, err := scrubAssetValues(child, depth+1)
			if err != nil {
				return nil, err
			}
			v[i] = clean
		}
	}
	return value, nil
}
