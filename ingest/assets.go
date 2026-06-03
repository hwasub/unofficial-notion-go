package ingest

import (
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
	"github.com/hwasub/unofficial-notion-go/notionid"
)

type blockAsset struct {
	BlockID   string
	Source    string
	SignedURL string
}

// CollectAssets discovers the Notion-hosted assets referenced by a page,
// returning one AssetSnapshot per asset in a stable order: page content first
// (following collection-query result order for collection views), then any
// remaining blocks by ID, then custom emojis. Only Notion-hosted sources are
// included; assets without a usable source or signed URL are skipped.
func CollectAssets(recordMap NormalizedRecordMap, pageID string) []AssetSnapshot {
	blocks := recordMap.Block
	seen := map[string]struct{}{}
	orderedBlocks := []map[string]any{}
	addBlock := func(block map[string]any) {
		if block == nil {
			return
		}
		id := notionrecordmap.StringValue(block["id"])
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		orderedBlocks = append(orderedBlocks, block)
	}
	for _, id := range PageContentIDs(recordMap, pageID) {
		addBlock(blocks[id])
	}
	collectionOrder := CollectionQueryBlockIDs(recordMap, pageID)
	collectionOrderSet := map[string]struct{}{}
	for _, id := range collectionOrder {
		collectionOrderSet[id] = struct{}{}
	}
	collectionOrderEmitted := false
	keys := make([]string, 0, len(blocks))
	for key := range blocks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, ok := collectionOrderSet[key]; ok {
			if !collectionOrderEmitted {
				for _, id := range collectionOrder {
					addBlock(blocks[id])
				}
				collectionOrderEmitted = true
			}
			continue
		}
		addBlock(blocks[key])
	}

	assets := []AssetSnapshot{}
	for _, block := range orderedBlocks {
		for _, asset := range blockAssets(recordMap, block) {
			if asset.Source == "" && asset.SignedURL == "" {
				continue
			}
			assets = append(assets, AssetSnapshot{
				BlockID:   asset.BlockID,
				PageID:    notionid.NormalizeBlockID(pageID),
				Type:      notionrecordmap.StringValue(block["type"]),
				Source:    asset.Source,
				SignedURL: firstNonEmpty(asset.SignedURL, asset.Source),
				Filename:  filenameFromURL(firstNonEmpty(asset.SignedURL, asset.Source)),
			})
		}
	}
	assets = append(assets, customEmojiAssets(recordMap, pageID)...)
	return assets
}

// CollectionQueryBlockIDs returns the normalized block IDs of collection
// records referenced by the collection-view blocks on a page, in the order the
// queries report them. It is used to order collection rows consistently when
// collecting assets.
func CollectionQueryBlockIDs(recordMap NormalizedRecordMap, pageID string) []string {
	ids := []string{}
	for _, blockID := range PageContentIDs(recordMap, pageID) {
		block := recordMap.Block[blockID]
		blockType := notionrecordmap.StringValue(block["type"])
		if blockType != "collection_view" && blockType != "collection_view_page" {
			continue
		}
		collectionID := firstNonEmpty(
			notionid.NormalizeBlockID(block["collection_id"]),
			notionid.NormalizeBlockID(collectionPointerID(block["format"])),
		)
		if collectionID == "" {
			continue
		}
		byCollection := notionrecordmap.AsMap(recordMap.CollectionQuery)
		viewQueries := notionrecordmap.AsMap(byCollection[collectionID])
		if viewQueries == nil {
			continue
		}
		for _, rawViewID := range notionrecordmap.AsSlice(block["view_ids"]) {
			viewID := notionid.NormalizeBlockID(rawViewID)
			if viewID == "" {
				continue
			}
			query := notionrecordmap.AsMap(viewQueries[viewID])
			if query == nil {
				continue
			}
			ids = append(ids, collectionQueryResultBlockIDs(query)...)
		}
	}
	return ids
}

func collectionQueryResultBlockIDs(query map[string]any) []string {
	out := []string{}
	add := func(value any) {
		groupResults := notionrecordmap.AsMap(value)
		for _, rawID := range notionrecordmap.AsSlice(groupResults["blockIds"]) {
			if id := notionid.NormalizeBlockID(rawID); id != "" {
				out = append(out, id)
			}
		}
	}
	add(query["collection_group_results"])
	keys := make([]string, 0, len(query))
	for key := range query {
		if strings.HasPrefix(key, "results:") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		add(query[key])
	}
	return out
}

func blockAssets(recordMap NormalizedRecordMap, block map[string]any) []blockAsset {
	propertyAssets := propertyFileAssets(recordMap, block)
	blockType := notionrecordmap.StringValue(block["type"])
	switch blockType {
	case "page":
		assets := append([]blockAsset{}, propertyAssets...)
		format := notionrecordmap.AsMap(block["format"])
		if cover := notionHostedAssetSourceValue(format["page_cover"]); cover != "" {
			assets = append(assets, blockAsset{
				BlockID:   notionrecordmap.StringValue(block["id"]),
				Source:    cover,
				SignedURL: signedURLForBlock(recordMap, block),
			})
		}
		if icon := notionHostedAssetSourceValue(format["page_icon"]); icon != "" {
			assets = append(assets, blockAsset{
				BlockID:   notionrecordmap.StringValue(block["id"]) + ":icon",
				Source:    icon,
				SignedURL: firstNonEmpty(signedURLForBlockRole(recordMap, block, "icon"), notionImageProxyURL(block, icon)),
			})
		}
		return assets
	case "callout":
		return append(roleVisualAssets(recordMap, block, []roleVisualAsset{{FormatKey: "page_icon", Role: "icon"}}), propertyAssets...)
	case "bookmark", "embed", "external_object_instance":
		return append(roleVisualAssets(recordMap, block, []roleVisualAsset{
			{FormatKey: "bookmark_icon", Role: "icon"},
			{FormatKey: "bookmark_cover", Role: "cover"},
		}), propertyAssets...)
	}
	switch blockType {
	case "image", "file", "pdf", "video", "audio":
	default:
		return propertyAssets
	}
	properties := notionrecordmap.AsMap(block["properties"])
	source := notionHostedAssetSourceValue(firstPropertyText(properties["source"]))
	signedURL := notionHostedAssetSourceValue(signedURLForBlock(recordMap, block))
	if source == "" && signedURL == "" {
		return propertyAssets
	}
	return append([]blockAsset{{BlockID: notionrecordmap.StringValue(block["id"]), Source: source, SignedURL: signedURL}}, propertyAssets...)
}

type roleVisualAsset struct {
	FormatKey string
	Role      string
}

func roleVisualAssets(recordMap NormalizedRecordMap, block map[string]any, roles []roleVisualAsset) []blockAsset {
	format := notionrecordmap.AsMap(block["format"])
	assets := []blockAsset{}
	for _, role := range roles {
		source := notionHostedAssetSourceValue(format[role.FormatKey])
		if source == "" {
			continue
		}
		assets = append(assets, blockAsset{
			BlockID: notionrecordmap.StringValue(block["id"]) + ":" + role.Role,
			Source:  source,
			SignedURL: firstNonEmpty(
				notionHostedAssetSourceValue(signedURLForBlockRole(recordMap, block, role.Role)),
				notionHostedAssetSourceValue(signedURLForSource(recordMap, source)),
				notionImageProxyURL(block, source),
			),
		})
	}
	return assets
}

func signedURLForBlock(recordMap NormalizedRecordMap, block map[string]any) string {
	id := notionrecordmap.StringValue(block["id"])
	normalized := notionid.NormalizeBlockID(id)
	compact := notionid.CompactID(id)
	for _, key := range []string{id, normalized, compact} {
		if value := recordMap.SignedURLs[key]; value != "" {
			return value
		}
	}
	return ""
}

func signedURLForBlockRole(recordMap NormalizedRecordMap, block map[string]any, role string) string {
	id := notionrecordmap.StringValue(block["id"])
	normalized := notionid.NormalizeBlockID(id)
	compact := notionid.CompactID(id)
	for _, key := range []string{id + ":" + role, normalized + ":" + role, compact + ":" + role} {
		if value := recordMap.SignedURLs[key]; value != "" {
			return value
		}
	}
	return ""
}

func propertyFileAssets(recordMap NormalizedRecordMap, block map[string]any) []blockAsset {
	properties := notionrecordmap.AsMap(block["properties"])
	if properties == nil {
		return nil
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		if key == "source" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	assets := []blockAsset{}
	for _, property := range keys {
		for index, source := range richTextAssetSources(properties[property]) {
			assets = append(assets, blockAsset{
				BlockID:   notionrecordmap.StringValue(block["id"]) + ":" + property + ":" + strconvItoa(index),
				Source:    source,
				SignedURL: notionHostedAssetSourceValue(signedURLForSource(recordMap, source)),
			})
		}
	}
	return assets
}

func richTextAssetSources(value any) []string {
	candidates := []string{}
	for _, rawPart := range notionrecordmap.AsSlice(value) {
		part := notionrecordmap.AsSlice(rawPart)
		if len(part) == 0 {
			continue
		}
		if text, ok := part[0].(string); ok {
			candidates = append(candidates, text)
		}
		decorations := []any{}
		if len(part) > 1 {
			decorations = notionrecordmap.AsSlice(part[1])
		}
		for _, rawDecoration := range decorations {
			decoration := notionrecordmap.AsSlice(rawDecoration)
			if len(decoration) < 2 {
				continue
			}
			if notionrecordmap.StringValue(decoration[0]) == "a" {
				if href, ok := decoration[1].(string); ok {
					candidates = append(candidates, href)
				}
			} else if notionrecordmap.StringValue(decoration[0]) == "lm" {
				linkMention := notionrecordmap.AsMap(decoration[1])
				candidates = append(candidates, notionrecordmap.StringValue(linkMention["icon_url"]))
				candidates = append(candidates, notionrecordmap.StringValue(linkMention["thumbnail_url"]))
			}
		}
	}
	out := []string{}
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		for _, raw := range strings.Split(candidate, ",") {
			source := notionHostedAssetSourceValue(raw)
			if source == "" {
				continue
			}
			if _, ok := seen[source]; ok {
				continue
			}
			seen[source] = struct{}{}
			out = append(out, source)
		}
	}
	return out
}

func customEmojiAssets(recordMap NormalizedRecordMap, pageID string) []AssetSnapshot {
	assets := []AssetSnapshot{}
	keys := make([]string, 0, len(recordMap.CustomEmojis))
	for id := range recordMap.CustomEmojis {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		source := notionHostedAssetSourceValue(recordMap.CustomEmojis[id])
		if source == "" {
			continue
		}
		signedURL := firstNonEmpty(notionHostedAssetSourceValue(signedURLForSource(recordMap, source)), source)
		assets = append(assets, AssetSnapshot{
			BlockID:   "custom_emoji:" + id,
			PageID:    notionid.NormalizeBlockID(pageID),
			Type:      "custom_emoji",
			Source:    source,
			SignedURL: signedURL,
			Filename:  filenameFromURL(firstNonEmpty(signedURL, source)),
		})
	}
	return assets
}

func signedURLForSource(recordMap NormalizedRecordMap, source string) string {
	if value := recordMap.SignedURLs[source]; value != "" {
		return value
	}
	return recordMap.SignedURLs[notionid.NormalizeBlockID(source)]
}

func notionHostedAssetSourceValue(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "attachment:") {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "/images/") || strings.HasPrefix(trimmed, "/icons/") {
		return "https://www.notion.so" + trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if isNotionAssetURL(parsed) {
		return trimmed
	}
	return ""
}

func notionImageProxyURL(block map[string]any, source string) string {
	parsed, err := url.Parse(source)
	if err != nil || !isNotionAssetURL(parsed) {
		return ""
	}
	id := notionid.NormalizeBlockID(block["id"])
	if id == "" {
		return ""
	}
	encodedSource := strings.ReplaceAll(url.QueryEscape(source), "+", "%20")
	out := "https://www.notion.so/image/" + encodedSource + "?table=block&id=" + url.QueryEscape(id)
	if spaceID := notionrecordmap.StringValue(block["space_id"]); spaceID != "" {
		out += "&spaceId=" + url.QueryEscape(spaceID)
	}
	out += "&cache=v2"
	return out
}

func isNotionAssetURL(parsed *url.URL) bool {
	host := strings.ToLower(parsed.Hostname())
	pathValue := strings.ToLower(parsed.EscapedPath())
	if (host == "notion.so" || host == "www.notion.so") &&
		(strings.HasPrefix(pathValue, "/images/") || strings.HasPrefix(pathValue, "/icons/")) {
		return true
	}
	if (host == "s3.us-west-2.amazonaws.com" || host == "s3-us-west-2.amazonaws.com") &&
		strings.Contains(pathValue, "/secure.notion-static.com/") {
		return true
	}
	switch host {
	case "secure.notion-static.com", "prod-files-secure.s3.us-west-2.amazonaws.com", "file.notion.so":
		return true
	default:
		return false
	}
}

func filenameFromURL(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "attachment:") {
		parts := strings.Split(value, ":")
		return decodeURLComponent(parts[len(parts)-1])
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return ""
	}
	if (parsed.Hostname() == "notion.so" || parsed.Hostname() == "www.notion.so") && strings.HasPrefix(parsed.EscapedPath(), "/image/") {
		target := decodeURLComponent(strings.TrimPrefix(parsed.EscapedPath(), "/image/"))
		if target != "" && target != value {
			return filenameFromURL(target)
		}
	}
	return decodeURLComponent(path.Base(parsed.EscapedPath()))
}

func decodeURLComponent(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstPropertyText(value any) string {
	parts := notionrecordmap.AsSlice(value)
	if len(parts) == 0 {
		return ""
	}
	first := notionrecordmap.AsSlice(parts[0])
	if len(first) == 0 {
		return ""
	}
	return notionrecordmap.StringValue(first[0])
}

func strconvItoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
