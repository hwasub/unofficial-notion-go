package ingest

import (
	"reflect"
	"testing"
)

func TestCollectAssetsResolvesSignedImageURLs(t *testing.T) {
	pageID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	imageID := "1c06e61c-f824-8035-bf63-c471d7188978"
	recordMap := NormalizeRecordMap(map[string]any{
		"block": map[string]any{
			pageID: map[string]any{
				"value": map[string]any{
					"id":         pageID,
					"type":       "page",
					"content":    []any{imageID},
					"properties": map[string]any{"title": []any{[]any{"Root"}}},
				},
			},
			imageID: map[string]any{
				"value": map[string]any{
					"id":         imageID,
					"type":       "image",
					"properties": map[string]any{"source": []any{[]any{"attachment:file.jpg"}}},
				},
			},
		},
		"signed_urls": map[string]any{
			imageID: "https://file.notion.so/file.jpg?signature=test",
		},
	})
	got := CollectAssets(recordMap, pageID)
	want := []AssetSnapshot{{
		BlockID:   imageID,
		PageID:    pageID,
		Type:      "image",
		Source:    "attachment:file.jpg",
		SignedURL: "https://file.notion.so/file.jpg?signature=test",
		Filename:  "file.jpg",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectAssets = %#v, want %#v", got, want)
	}
}

func TestCollectAssetsIncludesRoleVisualAssets(t *testing.T) {
	pageID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	calloutID := "2ad6e61c-f824-80c9-a6c4-d251043457d3"
	icon := "https://prod-files-secure.s3.us-west-2.amazonaws.com/workspace/callout.png"
	recordMap := NormalizeRecordMap(map[string]any{
		"block": map[string]any{
			pageID: map[string]any{"value": map[string]any{"id": pageID, "type": "page", "content": []any{calloutID}}},
			calloutID: map[string]any{
				"value": map[string]any{
					"id":         calloutID,
					"type":       "callout",
					"properties": map[string]any{"title": []any{[]any{"Callout"}}},
					"format":     map[string]any{"page_icon": icon},
				},
			},
		},
		"signed_urls": map[string]any{
			calloutID + ":icon": "https://file.notion.so/workspace/callout.png?signature=test",
		},
	})
	got := CollectAssets(recordMap, pageID)
	want := []AssetSnapshot{{
		BlockID:   calloutID + ":icon",
		PageID:    pageID,
		Type:      "callout",
		Source:    icon,
		SignedURL: "https://file.notion.so/workspace/callout.png?signature=test",
		Filename:  "callout.png",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CollectAssets = %#v, want %#v", got, want)
	}
}

func TestCollectAssetsUsesCollectionQueryOrderBeforeSortedFallback(t *testing.T) {
	pageID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionBlockID := "2ad6e61c-f824-80c9-a6c4-d251043457d3"
	contentImageID := "9ad6e61c-f824-80c9-a6c4-d251043457d3"
	rowA := "3ad6e61c-f824-80c9-a6c4-d251043457d3"
	rowB := "4ad6e61c-f824-80c9-a6c4-d251043457d3"
	collectionID := "5ad6e61c-f824-80c9-a6c4-d251043457d3"
	viewID := "6ad6e61c-f824-80c9-a6c4-d251043457d3"
	coverA := "https://www.notion.so/images/page-cover/solid_blue.png"
	coverB := "https://www.notion.so/images/page-cover/solid_red.png"
	contentImage := "https://www.notion.so/images/page-cover/solid_yellow.png"
	recordMap := NormalizeRecordMap(map[string]any{
		"block": map[string]any{
			pageID:            map[string]any{"value": map[string]any{"id": pageID, "type": "page", "content": []any{collectionBlockID, contentImageID}}},
			collectionBlockID: map[string]any{"value": map[string]any{"id": collectionBlockID, "type": "collection_view", "collection_id": collectionID, "view_ids": []any{viewID}}},
			contentImageID:    map[string]any{"value": map[string]any{"id": contentImageID, "type": "image", "properties": map[string]any{"source": []any{[]any{contentImage}}}}},
			rowA:              map[string]any{"value": map[string]any{"id": rowA, "type": "page", "format": map[string]any{"page_cover": coverA}}},
			rowB:              map[string]any{"value": map[string]any{"id": rowB, "type": "page", "format": map[string]any{"page_cover": coverB}}},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []any{rowB, rowA}},
				},
			},
		},
	})
	got := CollectAssets(recordMap, pageID)
	gotIDs := []string{}
	for _, asset := range got {
		gotIDs = append(gotIDs, asset.BlockID)
	}
	wantIDs := []string{contentImageID, rowB, rowA}
	if !reflect.DeepEqual(gotIDs, wantIDs) {
		t.Fatalf("asset block order = %#v, want %#v", gotIDs, wantIDs)
	}
}
