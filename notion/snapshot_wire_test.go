package notion_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hwasub/unofficial-notion-go/ingest"
	"github.com/hwasub/unofficial-notion-go/notion"
)

func TestIngestAndRendererSnapshotWireContract(t *testing.T) {
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	if ingest.SnapshotSchemaVersion != notion.SnapshotSchemaVersion {
		t.Fatalf("snapshot schema versions drifted: ingest=%d notion=%d", ingest.SnapshotSchemaVersion, notion.SnapshotSchemaVersion)
	}
	source := ingest.Snapshot{
		SchemaVersion: ingest.SnapshotSchemaVersion,
		FetchedAt:     "2026-07-10T12:00:00Z",
		RootPageID:    rootID,
		SourceURL:     "https://www.notion.so/example",
		Limits:        ingest.SnapshotLimits{MaxAssets: 3, MaxBlocks: 100},
		Page: ingest.PageSnapshot{
			PageID: rootID,
			Title:  "Root",
			RecordMap: ingest.NormalizedRecordMap{
				Block:            map[string]map[string]any{rootID: {"id": rootID, "type": "page"}},
				Collection:       map[string]map[string]any{},
				CollectionView:   map[string]map[string]any{},
				CollectionQuery:  map[string]any{},
				Automation:       map[string]map[string]any{},
				AutomationAction: map[string]map[string]any{},
				CustomEmojis:     map[string]string{},
				SignedURLs:       map[string]string{},
			},
			AssetCount: 1,
		},
		Assets:    []ingest.AssetSnapshot{{BlockID: rootID + ":icon", PageID: rootID, Type: "page", Source: "attachment:icon.png", SignedURL: "https://file.notion.so/signed", Filename: "icon.png"}},
		Errors:    []ingest.SnapshotError{{PageID: rootID, Message: "partial collection"}},
		Truncated: ingest.SnapshotFlags{Assets: true, Blocks: false, Collections: true},
	}
	wire, err := json.Marshal(source)
	if err != nil {
		t.Fatal(err)
	}
	var mirror notion.Snapshot
	if err := json.Unmarshal(wire, &mirror); err != nil {
		t.Fatal(err)
	}
	if err := notion.ValidateSnapshot(&mirror, rootID); err != nil {
		t.Fatalf("transported snapshot validation failed: %v", err)
	}
	roundTrip, err := json.Marshal(mirror)
	if err != nil {
		t.Fatal(err)
	}
	var before, after any
	if err := json.Unmarshal(wire, &before); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundTrip, &after); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("snapshot wire shape drifted\nbefore: %s\nafter:  %s", wire, roundTrip)
	}
}
