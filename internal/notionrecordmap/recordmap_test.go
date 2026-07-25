package notionrecordmap

import (
	"reflect"
	"testing"
)

func TestGetPageContentBlockIDsFollowsPointersInEveryRichTextPartAndDecoration(t *testing.T) {
	const (
		rootID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		firstID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		secondID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		thirdID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		contentID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		nestedID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := map[string]any{
		"block": map[string]any{
			rootID: map[string]any{"value": map[string]any{
				"id":   rootID,
				"type": "page",
				"properties": map[string]any{
					"title": []any{
						[]any{"plain", []any{[]any{"b"}}},
						[]any{"two pointers", []any{
							[]any{"i"},
							[]any{"p", firstID},
							[]any{"p", secondID},
						}},
						[]any{"later part", []any{[]any{"p", thirdID}}},
					},
				},
				"content": []any{contentID},
			}},
			firstID:  map[string]any{"value": map[string]any{"id": firstID, "type": "text"}},
			secondID: map[string]any{"value": map[string]any{"id": secondID, "type": "text"}},
			thirdID: map[string]any{"value": map[string]any{
				"id":      thirdID,
				"type":    "page",
				"content": []any{nestedID},
			}},
			contentID: map[string]any{"value": map[string]any{"id": contentID, "type": "text"}},
			nestedID:  map[string]any{"value": map[string]any{"id": nestedID, "type": "text"}},
		},
	}

	got := GetPageContentBlockIDs(recordMap, rootID)
	want := []string{rootID, firstID, secondID, thirdID, contentID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetPageContentBlockIDs() = %#v, want %#v", got, want)
	}
}

func TestGetPageContentBlockIDsFollowsAliasPointersWithoutCycling(t *testing.T) {
	const (
		rootID      = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		aliasID     = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		targetID    = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		targetChild = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		ordinaryID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := map[string]any{
		"block": map[string]any{
			rootID: map[string]any{"value": map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []any{aliasID, ordinaryID},
			}},
			aliasID: map[string]any{"value": map[string]any{
				"id":   aliasID,
				"type": "alias",
				"format": map[string]any{
					"alias_pointer": map[string]any{"id": targetID},
				},
			}},
			targetID: map[string]any{"value": map[string]any{
				"id":      targetID,
				"type":    "text",
				"content": []any{targetChild},
				"format": map[string]any{
					"alias_pointer": map[string]any{"id": aliasID},
				},
			}},
			targetChild: map[string]any{"value": map[string]any{
				"id":   targetChild,
				"type": "text",
			}},
			ordinaryID: map[string]any{"value": map[string]any{
				"id":   ordinaryID,
				"type": "text",
			}},
		},
	}

	got := GetPageContentBlockIDs(recordMap, rootID)
	want := []string{rootID, aliasID, targetID, targetChild, ordinaryID}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GetPageContentBlockIDs() = %#v, want %#v", got, want)
	}
}
