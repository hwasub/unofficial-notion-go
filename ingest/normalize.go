package ingest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
	"github.com/hwasub/unofficial-notion-go/notionid"
)

var scrubKeys = map[string]struct{}{
	"__proto__":            {},
	"constructor":          {},
	"permissions":          {},
	"version":              {},
	"prototype":            {},
	"created_by":           {},
	"created_by_id":        {},
	"last_edited_by":       {},
	"last_edited_by_id":    {},
	"notion_user":          {},
	"discussions":          {},
	"comments":             {},
	"file_upload":          {},
	"copied_from_pointer":  {},
	"created_by_table":     {},
	"last_edited_by_table": {},
}

// NormalizeRecordMap converts a raw Notion record map into a NormalizedRecordMap:
// it unboxes and re-keys records by normalized block ID, scrubs sensitive and
// volatile fields, prunes blocks missing a type or ID, and normalizes
// automations, custom emojis, and signed URLs. It accepts any shape and treats
// a nil or non-map input as empty.
func NormalizeRecordMap(recordMap any) NormalizedRecordMap {
	source := notionrecordmap.AsMap(recordMap)
	if source == nil {
		source = map[string]any{}
	}
	return NormalizedRecordMap{
		Block:            pruneDanglingBlocks(normalizeMap(source["block"])),
		Collection:       normalizeMap(source["collection"]),
		CollectionView:   normalizeMap(source["collection_view"]),
		CollectionQuery:  Scrub(defaultMap(source["collection_query"])),
		Automation:       normalizeAutomationMap(source["automation"]),
		AutomationAction: normalizeAutomationActionMap(source["automation_action"]),
		CustomEmojis:     normalizeCustomEmojis(source["custom_emojis"]),
		SignedURLs:       normalizeSignedURLs(source["signed_urls"]),
	}
}

// Scrub returns a deep copy of value with disallowed keys removed from every
// nested map. It drops prototype-pollution keys and sensitive or volatile
// Notion fields (see scrubKeys), and sorts map keys for deterministic output.
// Slices are recursed into and scalar values are returned unchanged.
func Scrub(value any) any {
	switch typed := value.(type) {
	case []any:
		out := make([]any, len(typed))
		for i, child := range typed {
			out[i] = Scrub(child)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if _, drop := scrubKeys[key]; drop {
				continue
			}
			out[key] = Scrub(typed[key])
		}
		return out
	default:
		return value
	}
}

func normalizeMap(value any) map[string]map[string]any {
	out := map[string]map[string]any{}
	source := notionrecordmap.AsMap(value)
	if source == nil {
		return out
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		entry := source[id]
		unboxed := notionrecordmap.AsMap(notionrecordmap.Unbox(entry))
		if unboxed == nil {
			continue
		}
		normalizedID := notionid.NormalizeBlockID(firstValue(unboxed["id"], id))
		scrubbed := notionrecordmap.AsMap(Scrub(unboxed))
		if scrubbed == nil {
			continue
		}
		scrubbed["id"] = normalizedID
		if notionrecordmap.StringValue(scrubbed["id"]) != "" {
			out[normalizedID] = scrubbed
		}
	}
	return out
}

func normalizeAutomationMap(value any) map[string]map[string]any {
	out := map[string]map[string]any{}
	source := notionrecordmap.AsMap(value)
	if source == nil {
		return out
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		value := notionrecordmap.AsMap(notionrecordmap.Unbox(source[id]))
		if value == nil {
			continue
		}
		normalizedID := notionid.NormalizeBlockID(firstValue(value["id"], id))
		if normalizedID == "" {
			continue
		}
		actionIDs := []any{}
		for _, raw := range notionrecordmap.AsSlice(value["action_ids"]) {
			if id := notionid.NormalizeBlockID(raw); id != "" {
				actionIDs = append(actionIDs, id)
			}
		}
		normalized := map[string]any{"id": normalizedID, "action_ids": actionIDs}
		properties := notionrecordmap.AsMap(value["properties"])
		normalizedProperties := map[string]any{}
		if name := notionrecordmap.StringValue(properties["name"]); name != "" {
			normalizedProperties["name"] = name
		}
		if icon := notionrecordmap.StringValue(properties["icon"]); icon != "" {
			normalizedProperties["icon"] = icon
		}
		if len(normalizedProperties) > 0 {
			normalized["properties"] = normalizedProperties
		}
		if trigger := notionrecordmap.AsMap(value["trigger"]); trigger != nil {
			if triggerType := notionrecordmap.StringValue(trigger["type"]); triggerType != "" {
				normalized["trigger"] = map[string]any{"type": triggerType}
			}
		}
		out[normalizedID] = normalized
	}
	return out
}

func normalizeAutomationActionMap(value any) map[string]map[string]any {
	out := map[string]map[string]any{}
	source := notionrecordmap.AsMap(value)
	if source == nil {
		return out
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		value := notionrecordmap.AsMap(notionrecordmap.Unbox(source[id]))
		if value == nil {
			continue
		}
		normalizedID := notionid.NormalizeBlockID(firstValue(value["id"], id))
		actionType := notionrecordmap.StringValue(value["type"])
		if normalizedID == "" || actionType == "" {
			continue
		}
		normalized := map[string]any{"id": normalizedID, "type": actionType}
		if actionType == "open_page" {
			if config := normalizeOpenPageActionConfig(value["config"]); config != nil {
				normalized["config"] = config
			}
		}
		out[normalizedID] = normalized
	}
	return out
}

func normalizeOpenPageActionConfig(value any) map[string]any {
	config := notionrecordmap.AsMap(value)
	if config == nil {
		return nil
	}
	target := notionrecordmap.AsMap(config["target"])
	if target == nil {
		return nil
	}
	switch notionrecordmap.StringValue(target["type"]) {
	case "url":
		if rawURL := notionrecordmap.StringValue(target["url"]); rawURL != "" {
			return map[string]any{"target": map[string]any{"type": "url", "url": rawURL}}
		}
	case "page":
		if pageID := notionid.NormalizeBlockID(target["pageId"]); pageID != "" {
			return map[string]any{"target": map[string]any{"type": "page", "pageId": pageID}}
		}
	}
	return nil
}

func normalizeSignedURLs(value any) map[string]string {
	out := map[string]string{}
	source := notionrecordmap.AsMap(Scrub(value))
	if source == nil {
		return out
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if signed, ok := source[id].(string); ok {
			out[normalizeAssetKey(id)] = signed
		}
	}
	return out
}

func normalizeAssetKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	base, suffix, ok := strings.Cut(key, ":")
	base = notionid.NormalizeBlockID(base)
	if !ok {
		return base
	}
	return base + ":" + suffix
}

func normalizeCustomEmojis(value any) map[string]string {
	out := map[string]string{}
	source := notionrecordmap.AsMap(value)
	if source == nil {
		return out
	}
	keys := make([]string, 0, len(source))
	for key := range source {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if _, drop := scrubKeys[id]; drop {
			continue
		}
		normalizedID := notionid.NormalizeBlockID(id)
		if normalizedID == "" {
			normalizedID = strings.TrimSpace(id)
		}
		rawURL, ok := source[id].(string)
		if !ok || normalizedID == "" {
			continue
		}
		if trimmed := strings.TrimSpace(rawURL); trimmed != "" {
			out[normalizedID] = trimmed
		}
	}
	return out
}

func pruneDanglingBlocks(blocks map[string]map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	keys := make([]string, 0, len(blocks))
	for key := range blocks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, id := range keys {
		block := blocks[id]
		if notionrecordmap.StringValue(block["type"]) == "" || notionrecordmap.StringValue(block["id"]) == "" {
			continue
		}
		out[id] = block
	}
	return out
}

// GetTitle returns the plain-text title of a block, reading its "title"
// property. It returns "" for a nil block or a block without a title.
func GetTitle(block map[string]any) string {
	if block == nil {
		return ""
	}
	properties := notionrecordmap.AsMap(block["properties"])
	return PlainText(properties["title"])
}

// PlainText flattens a Notion rich-text property into its concatenated plain
// text, taking the leading text segment of each part, and returns the trimmed
// result. It returns "" when the property has no text parts.
func PlainText(property any) string {
	parts := notionrecordmap.AsSlice(property)
	if len(parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, partValue := range parts {
		part := notionrecordmap.AsSlice(partValue)
		if len(part) > 0 {
			b.WriteString(fmt.Sprint(part[0]))
		} else if partValue != nil {
			b.WriteString(fmt.Sprint(partValue))
		}
	}
	return strings.TrimSpace(b.String())
}

// PageContentIDs returns the normalized IDs of the blocks reachable from the
// given page, in depth-first content order starting at the page itself.
// Duplicate and missing blocks are skipped. It returns an empty slice when the
// page is not present in the record map.
func PageContentIDs(recordMap NormalizedRecordMap, pageID string) []string {
	blocks := recordMap.Block
	rootID := notionid.NormalizeBlockID(pageID)
	seen := map[string]struct{}{}
	out := []string{}
	var walk func(any)
	walk = func(value any) {
		id := notionid.NormalizeBlockID(value)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		block := blocks[id]
		if block == nil {
			return
		}
		out = append(out, id)
		for _, child := range notionrecordmap.AsSlice(block["content"]) {
			walk(child)
		}
	}
	if blocks[rootID] != nil {
		walk(rootID)
	}
	return out
}

// SubpageLink identifies a child page found within a page's record map by its
// normalized page ID and plain-text title.
type SubpageLink struct {
	PageID string `json:"page_id"`
	Title  string `json:"title"`
}

// SubpageLinks returns the page blocks in the record map other than the given
// root page, as SubpageLinks sorted by block key. It is used to surface links
// to nested pages.
func SubpageLinks(recordMap NormalizedRecordMap, pageID string) []SubpageLink {
	rootCompact := notionid.CompactID(pageID)
	keys := make([]string, 0, len(recordMap.Block))
	for key := range recordMap.Block {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := []SubpageLink{}
	for _, key := range keys {
		block := recordMap.Block[key]
		if notionrecordmap.StringValue(block["type"]) != "page" {
			continue
		}
		if notionid.CompactID(block["id"]) == rootCompact {
			continue
		}
		out = append(out, SubpageLink{PageID: notionid.NormalizeBlockID(block["id"]), Title: GetTitle(block)})
	}
	return out
}

func firstValue(values ...any) any {
	for _, value := range values {
		if notionrecordmap.StringValue(value) != "" {
			return value
		}
	}
	return ""
}

func defaultMap(value any) any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
