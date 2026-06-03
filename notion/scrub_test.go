package notion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestScrubSnapshotForStorageRemovesSignedURLs(t *testing.T) {
	recordMap := json.RawMessage(`{
		"block": {
			"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {
				"value": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}
			}
		},
		"collection": {"keep": true},
		"signed_urls": {
			"https://secure.notion-static.com/image.png": "https://signed.example/image.png?X-Amz-Signature=secret"
		}
	}`)
	snapshot := &Snapshot{
		RootPageID: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		Page: PageSnapshot{
			PageID:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			RecordMap: recordMap,
		},
		Assets: []AssetSnapshot{{
			BlockID:   "CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC",
			PageID:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Source:    "https://secure.notion-static.com/image.png",
			SignedURL: "https://signed.example/image.png?token=secret",
			Filename:  "image.png",
		}, {
			BlockID:   "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD:icon",
			PageID:    "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			Source:    "https://secure.notion-static.com/icon.png",
			SignedURL: "https://signed.example/icon.png?token=secret",
			Filename:  "icon.png",
		}},
	}

	got, err := ScrubSnapshotForStorage(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("ScrubSnapshotForStorage returned nil")
	}
	storedRecordMap := string(got.Page.RecordMap)
	if strings.Contains(storedRecordMap, "signed_urls") || strings.Contains(storedRecordMap, "X-Amz-Signature") {
		t.Fatalf("stored record map still contains signed URL data: %s", storedRecordMap)
	}
	var decoded map[string]any
	if err := json.Unmarshal(got.Page.RecordMap, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["block"] == nil || decoded["collection"] == nil {
		t.Fatalf("stored record map lost unsigned content: %+v", decoded)
	}
	if got.Assets[0].SignedURL != "" {
		t.Fatalf("asset SignedURL = %q, want empty", got.Assets[0].SignedURL)
	}
	if snapshot.Assets[0].SignedURL == "" {
		t.Fatal("ScrubSnapshotForStorage mutated the input asset")
	}
	if got.RootPageID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("RootPageID = %q, want normalized id", got.RootPageID)
	}
	if got.Page.PageID != "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa" {
		t.Fatalf("Page.PageID = %q, want normalized id", got.Page.PageID)
	}
	if got.Assets[0].BlockID != "cccccccc-cccc-cccc-cccc-cccccccccccc" {
		t.Fatalf("asset BlockID = %q, want normalized id", got.Assets[0].BlockID)
	}
	if got.Assets[1].BlockID != "dddddddd-dddd-dddd-dddd-dddddddddddd:icon" {
		t.Fatalf("role asset BlockID = %q, want normalized asset key", got.Assets[1].BlockID)
	}
}
