package notion

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStorageScrubsEmbeddedNotionCredentialsWithoutChangingExternalLinks(t *testing.T) {
	const signed = "https://file.notion.com/f/f/space/file/image.png?signature=secret"
	const external = "https://example.com/resource?token=keep"
	raw, _ := json.Marshal(map[string]any{"block": map[string]any{"page": map[string]any{"format": map[string]any{"page_cover": signed}, "properties": map[string]any{"source": [][]string{{signed}}, "title": [][]string{{external}}}}}})
	snapshot := &Snapshot{Page: PageSnapshot{RecordMap: raw}, Assets: []AssetSnapshot{{Source: signed, SignedURL: signed}}}
	clean, err := ScrubSnapshotForStorage(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(clean)
	if strings.Contains(string(encoded), "secret") || clean.Assets[0].Source != "attachment:file:image.png" || !strings.Contains(string(clean.Page.RecordMap), external) {
		t.Fatalf("invalid scrub: %s", encoded)
	}
	if !strings.Contains(string(snapshot.Page.RecordMap), "secret") {
		t.Fatal("mutated input")
	}
	if got := safeExternalImageURL(signed); got != "" {
		t.Fatalf("signed Notion URL treated as external: %q", got)
	}
	if got := assetAliasURL(RenderInput{AssetURLs: map[string]string{"attachment:file:image.png": "https://cdn.example/image.png"}}, signed); got != "https://cdn.example/image.png" {
		t.Fatalf("stable mapping lost: %q", got)
	}
}

func TestCollectionExplicitCoverAndOptionContracts(t *testing.T) {
	const root = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const imageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	row := block{ID: root, Content: []string{imageID}}
	rm := recordMap{Block: map[string]block{imageID: {ID: imageID, Type: "image"}}}
	input := RenderInput{AssetURLs: map[string]string{imageID: "https://cdn.example/image.png"}}
	for _, kind := range []string{"page_content", "page_content_first"} {
		if got := collectionCoverHTML(rm, row, input, map[string]any{"type": kind}); !strings.Contains(got, "https://cdn.example/image.png") {
			t.Fatalf("missing %s cover: %s", kind, got)
		}
	}
	if got := collectionCoverHTML(rm, row, input, map[string]any{"type": "none"}); got != "" {
		t.Fatal("none cover rendered")
	}
	for _, fixture := range []struct {
		options string
		ghost   bool
	}{{``, true}, {`,"options":[]`, false}, {`,"options":[{"value":"Valid"}]`, false}} {
		var schema collectionProperty
		if err := json.Unmarshal([]byte(`{"type":"multi_select"`+fixture.options+`}`), &schema); err != nil {
			t.Fatal(err)
		}
		got := renderCollectionPills([]string{"Valid", "Ghost"}, schema)
		if strings.Contains(got, "Ghost") != fixture.ghost {
			t.Fatalf("options %s: %s", fixture.options, got)
		}
	}
}

func TestCollectionHiddenTitlesForSingleAndMultipleViews(t *testing.T) {
	const root = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	for _, multiple := range []bool{false, true} {
		views := []string{"v1"}
		if multiple {
			views = append(views, "v2")
		}
		raw := marshalRecordMapObject(t, map[string]any{
			"block":           map[string]any{root: map[string]any{"id": root, "type": "collection_view", "collection_id": "c", "view_ids": views, "format": map[string]any{"hide_inline_collection_name": true}}},
			"collection":      map[string]any{"c": map[string]any{"id": "c", "name": [][]string{{"Hidden title"}}}},
			"collection_view": map[string]any{"v1": map[string]any{"id": "v1", "type": "gallery"}, "v2": map[string]any{"id": "v2", "type": "table"}},
		})
		html, err := RenderPage(RenderInput{RecordMap: raw, PageID: root})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(html, "Hidden title") {
			t.Fatalf("hidden title rendered: %s", html)
		}
	}
}
