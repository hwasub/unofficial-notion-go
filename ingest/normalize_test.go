package ingest

import (
	"reflect"
	"testing"
)

func TestNormalizeRecordMapScrubsPrivateMetadata(t *testing.T) {
	pageID := "1ad6e61c-f824-80c9-a6c4-d251043457d3"
	childID := "878d7114-b59d-453e-adbb-d56faf9c66d4"
	normalized := NormalizeRecordMap(map[string]any{
		"block": map[string]any{
			"1ad6e61cf82480c9a6c4d251043457d3": map[string]any{
				"value": map[string]any{
					"value": map[string]any{
						"id":            pageID,
						"type":          "page",
						"properties":    map[string]any{"title": []any{[]any{"Root"}}},
						"content":       []any{childID},
						"permissions":   []any{map[string]any{"role": "reader"}},
						"created_by_id": "user-id",
					},
				},
			},
			"878d7114b59d453eadbbd56faf9c66d4": map[string]any{
				"value": map[string]any{
					"id":           childID,
					"type":         "page",
					"parent_id":    pageID,
					"parent_table": "block",
					"properties":   map[string]any{"title": []any{[]any{"Child"}}},
				},
			},
		},
	})
	root := normalized.Block[pageID]
	if root["permissions"] != nil || root["created_by_id"] != nil {
		t.Fatalf("private metadata was not scrubbed: %#v", root)
	}
	if title := GetTitle(root); title != "Root" {
		t.Fatalf("GetTitle = %q", title)
	}
	links := SubpageLinks(normalized, pageID)
	want := []SubpageLink{{PageID: childID, Title: "Child"}}
	if !reflect.DeepEqual(links, want) {
		t.Fatalf("SubpageLinks = %#v, want %#v", links, want)
	}
}

func TestNormalizeRecordMapKeepsOnlySafeAutomationData(t *testing.T) {
	automationID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	actionID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	normalized := NormalizeRecordMap(map[string]any{
		"automation": map[string]any{
			automationID: map[string]any{
				"value": map[string]any{
					"id":         automationID,
					"action_ids": []any{actionID},
					"properties": map[string]any{"name": "Open docs", "icon": "link", "secret": "drop"},
					"permissions": []any{
						map[string]any{"role": "reader"},
					},
				},
			},
		},
		"automation_action": map[string]any{
			actionID: map[string]any{
				"value": map[string]any{
					"id":   actionID,
					"type": "open_page",
					"config": map[string]any{
						"target":        map[string]any{"type": "url", "url": "https://example.com/docs"},
						"customHeaders": []any{map[string]any{"key": "Authorization", "value": "secret"}},
					},
				},
			},
		},
	})
	if got := normalized.Automation[automationID]["properties"]; !reflect.DeepEqual(got, map[string]any{"name": "Open docs", "icon": "link"}) {
		t.Fatalf("automation properties = %#v", got)
	}
	wantAction := map[string]any{
		"id":   actionID,
		"type": "open_page",
		"config": map[string]any{
			"target": map[string]any{"type": "url", "url": "https://example.com/docs"},
		},
	}
	if !reflect.DeepEqual(normalized.AutomationAction[actionID], wantAction) {
		t.Fatalf("automation action = %#v", normalized.AutomationAction[actionID])
	}
}

func TestNormalizeRecordMapNormalizesSignedURLRoleKeys(t *testing.T) {
	blockID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	normalized := NormalizeRecordMap(map[string]any{
		"signed_urls": map[string]any{
			"CCCCCCCCCCCCCCCCCCCCCCCCCCCCCCCC:icon": "https://file.notion.so/icon.png?signature=test",
			"https://file.notion.so/source.png":     "https://file.notion.so/source.png?signature=test",
		},
	})
	if got := normalized.SignedURLs[blockID+":icon"]; got == "" {
		t.Fatalf("role signed URL key was not normalized: %#v", normalized.SignedURLs)
	}
	if got := normalized.SignedURLs["https://file.notion.so/source.png"]; got == "" {
		t.Fatalf("source signed URL key was not preserved: %#v", normalized.SignedURLs)
	}
}
