// Package notionrecordmap provides helpers for navigating the loosely typed
// record map that Notion's private API returns. Values arrive as nested
// map[string]any and []any structures, so these helpers safely coerce, unbox,
// and walk them to extract block values, collection IDs, and content trees.
package notionrecordmap

import (
	"fmt"
	"strings"
)

// AsMap returns value as a map[string]any, or nil when it is not one.
func AsMap(value any) map[string]any {
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

// AsSlice returns value as a []any, or nil when it is not one.
func AsSlice(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

// StringValue returns value formatted as a trimmed string, or the empty string
// when value is nil.
func StringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// LooseStringValue returns value formatted as a string without trimming
// whitespace, or the empty string when value is nil.
func LooseStringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

// Unbox unwraps record-map envelopes by repeatedly following the "value" key
// until it reaches a non-map or a map without one. It returns the unwrapped
// value, or nil if the nesting exceeds the depth limit.
func Unbox(value any) any {
	current := value
	for depth := 0; depth < 64; depth++ {
		record, ok := current.(map[string]any)
		if !ok {
			return current
		}
		child, ok := record["value"]
		if !ok {
			return current
		}
		current = child
	}
	return nil
}

// GetBlockValue unboxes value and returns it as a block map, requiring an "id"
// field to be present. It returns nil when value is not a block.
func GetBlockValue(value any) map[string]any {
	unboxed := Unbox(value)
	record := AsMap(unboxed)
	if record == nil {
		return nil
	}
	if _, ok := record["id"]; !ok {
		return nil
	}
	return record
}

// GetBlockCollectionID resolves the collection ID for block, checking its
// collection_id and format pointer first and then falling back to the
// collection view referenced by the block's view_ids within recordMap. It
// returns the empty string when no collection ID can be determined.
func GetBlockCollectionID(block map[string]any, recordMap map[string]any) string {
	if block == nil {
		return ""
	}
	if id := StringValue(block["collection_id"]); id != "" {
		return id
	}
	if format := AsMap(block["format"]); format != nil {
		if pointer := AsMap(format["collection_pointer"]); pointer != nil {
			if id := StringValue(pointer["id"]); id != "" {
				return id
			}
		}
	}

	viewIDs := AsSlice(block["view_ids"])
	if len(viewIDs) == 0 {
		return ""
	}
	collectionViewID := StringValue(viewIDs[0])
	if collectionViewID == "" {
		return ""
	}
	collectionViews := AsMap(recordMap["collection_view"])
	if collectionViews == nil {
		return ""
	}
	collectionView := GetBlockValue(collectionViews[collectionViewID])
	if collectionView == nil {
		return ""
	}
	format := AsMap(collectionView["format"])
	if format == nil {
		return ""
	}
	pointer := AsMap(format["collection_pointer"])
	if pointer == nil {
		return ""
	}
	return StringValue(pointer["id"])
}

// GetPageContentBlockIDs walks recordMap starting at blockID (or the first
// block when blockID is empty) and returns the IDs of all reachable content
// blocks in traversal order, following content, property pointers, and
// transclusion references while stopping at nested pages.
func GetPageContentBlockIDs(recordMap map[string]any, blockID string) []string {
	blocks := AsMap(recordMap["block"])
	if len(blocks) == 0 {
		return nil
	}
	rootBlockID := blockID
	if rootBlockID == "" {
		for id := range blocks {
			rootBlockID = id
			break
		}
	}
	seen := map[string]struct{}{}
	out := []string{}

	var addContentBlocks func(string)
	addContentBlocks = func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)

		block := GetBlockValue(blocks[id])
		if block == nil {
			return
		}
		if properties := AsMap(block["properties"]); properties != nil {
			for _, property := range properties {
				for _, decoration := range AsSlice(property) {
					part := AsSlice(decoration)
					if len(part) > 0 {
						if value := firstPointerValue(part[0]); value != "" {
							addContentBlocks(value)
						}
					}
				}
				propertyParts := AsSlice(property)
				if len(propertyParts) > 0 {
					if value := firstPointerValue(propertyParts[0]); value != "" {
						addContentBlocks(value)
					}
				}
			}
		}
		if format := AsMap(block["format"]); format != nil {
			pointer := AsMap(format["transclusion_reference_pointer"])
			if pointer != nil {
				if value := StringValue(pointer["id"]); value != "" {
					addContentBlocks(value)
				}
			}
		}

		content := AsSlice(block["content"])
		if len(content) == 0 {
			return
		}
		if id != rootBlockID {
			switch StringValue(block["type"]) {
			case "page", "collection_view_page":
				return
			}
		}
		for _, child := range content {
			addContentBlocks(StringValue(child))
		}
	}

	addContentBlocks(rootBlockID)
	return out
}

func firstPointerValue(value any) string {
	items := AsSlice(value)
	if len(items) < 2 {
		return ""
	}
	nested := AsSlice(items[1])
	if len(nested) == 0 {
		return ""
	}
	pointer := AsSlice(nested[0])
	if len(pointer) > 1 && StringValue(pointer[0]) == "p" {
		return StringValue(pointer[1])
	}
	return ""
}

// MapKeys returns the keys of value in unspecified order.
func MapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	return keys
}
