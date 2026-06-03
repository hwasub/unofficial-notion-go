package ingest

import (
	"context"
	"sort"
	"strings"

	"github.com/hwasub/unofficial-notion-go/internal/notionid"
	"github.com/hwasub/unofficial-notion-go/internal/notionrecordmap"
	"github.com/hwasub/unofficial-notion-go/notionapi"
)

func repairCollectionQueries(ctx context.Context, client PageClient, recordMap map[string]any, log func(string, map[string]any)) error {
	blocks := notionrecordmap.AsMap(recordMap["block"])
	if blocks == nil {
		return nil
	}
	collectionQueries := ensureRecordMap(recordMap, "collection_query")
	keys := make([]string, 0, len(blocks))
	for key := range blocks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		block := notionrecordmap.AsMap(notionrecordmap.Unbox(blocks[key]))
		if block == nil {
			continue
		}
		blockType := notionrecordmap.StringValue(block["type"])
		if blockType != "collection_view" && blockType != "collection_view_page" {
			continue
		}
		collectionID := firstNonEmpty(notionrecordmap.StringValue(block["collection_id"]), collectionPointerID(block["format"]))
		viewIDs := arrayStrings(block["view_ids"])
		if collectionID == "" || len(viewIDs) == 0 {
			continue
		}
		for _, viewID := range viewIDs {
			collectionView := notionrecordmap.AsMap(notionrecordmap.Unbox(notionrecordmap.AsMap(recordMap["collection_view"])[viewID]))
			if !isRepairableCollectionView(collectionView) {
				continue
			}
			existing := notionrecordmap.AsMap(collectionQueries[collectionID])
			existingViewQuery := any(nil)
			if existing != nil {
				existingViewQuery = existing[viewID]
			}
			if !collectionQueryNeedsRepair(existingViewQuery, collectionView, recordMap) {
				continue
			}
			collectionData, err := client.GetCollectionData(ctx, collectionID, viewID, collectionView, notionapi.CollectionOptions{
				Limit: 999,
				SpaceID: firstNonEmpty(
					notionrecordmap.StringValue(block["space_id"]),
					collectionPointerSpaceID(block["format"]),
					collectionPointerSpaceID(collectionView["format"]),
				),
			})
			if err != nil {
				log("notion_collection_query_repair_failed", map[string]any{
					"page_id":       recordMapRootPageID(recordMap),
					"collection_id": collectionID,
					"view_id":       viewID,
					"view_type":     notionrecordmap.StringValue(collectionView["type"]),
					"message":       err.Error(),
				})
				continue
			}
			mergeRawRecordMap(recordMap, notionrecordmap.AsMap(collectionData["recordMap"]))
			reducerResults := notionrecordmap.AsMap(notionrecordmap.AsMap(collectionData["result"])["reducerResults"])
			if reducerResults == nil {
				if raw := notionrecordmap.AsMap(collectionData["result"])["reducerResults"]; raw != nil {
					byCollection := existingOrNew(collectionQueries, collectionID)
					byCollection[viewID] = raw
				}
				continue
			}
			byCollection := existingOrNew(collectionQueries, collectionID)
			previousQuery := notionrecordmap.AsMap(byCollection[viewID])
			if previousQuery == nil {
				previousQuery = map[string]any{}
			}
			for key, value := range reducerResults {
				previousQuery[key] = value
			}
			byCollection[viewID] = previousQuery
		}
	}
	return nil
}

func isRepairableCollectionView(value map[string]any) bool {
	switch notionrecordmap.StringValue(value["type"]) {
	case "board", "table", "list", "gallery", "calendar", "timeline":
		return true
	default:
		return false
	}
}

func collectionQueryNeedsRepair(query any, collectionView map[string]any, recordMap map[string]any) bool {
	queryMap := notionrecordmap.AsMap(query)
	if queryMap == nil {
		return true
	}
	if notionrecordmap.StringValue(collectionView["type"]) == "board" && notionrecordmap.AsMap(queryMap["board_columns"]) == nil {
		return true
	}
	groupResults := notionrecordmap.AsMap(queryMap["collection_group_results"])
	blockIDs := arrayStrings(groupResults["blockIds"])
	if len(blockIDs) == 0 {
		return true
	}
	for _, id := range blockIDs {
		if !recordMapHasBlock(recordMap, id) {
			return true
		}
	}
	if collectionViewHasAggregations(collectionView) && !collectionQueryHasAggregationResults(queryMap) {
		return true
	}
	if collectionViewHasGrouping(collectionView) && !collectionQueryHasGroupedResults(queryMap) {
		return true
	}
	return false
}

func collectionViewHasAggregations(collectionView map[string]any) bool {
	for _, source := range []map[string]any{
		notionrecordmap.AsMap(collectionView["format"]),
		notionrecordmap.AsMap(collectionView["query"]),
		notionrecordmap.AsMap(collectionView["query2"]),
	} {
		if source == nil {
			continue
		}
		if collectionAggregationConfigPresent(source["aggregations"]) || collectionAggregationConfigPresent(source["aggregate"]) {
			return true
		}
	}
	return false
}

func collectionAggregationConfigPresent(value any) bool {
	if items := notionrecordmap.AsSlice(value); len(items) > 0 {
		return true
	}
	return len(notionrecordmap.AsMap(value)) > 0
}

func collectionQueryHasAggregationResults(query map[string]any) bool {
	groupResults := notionrecordmap.AsMap(query["collection_group_results"])
	return len(notionrecordmap.AsSlice(groupResults["aggregationResults"])) > 0
}

func collectionViewHasGrouping(collectionView map[string]any) bool {
	for _, source := range []map[string]any{
		notionrecordmap.AsMap(collectionView["format"]),
		notionrecordmap.AsMap(collectionView["query"]),
		notionrecordmap.AsMap(collectionView["query2"]),
	} {
		for key, value := range source {
			if strings.Contains(strings.ToLower(key), "group") && notionrecordmap.LooseStringValue(value) != "" {
				return true
			}
		}
	}
	return false
}

func collectionQueryHasGroupedResults(query map[string]any) bool {
	for key := range query {
		if strings.HasPrefix(key, "results:") {
			return true
		}
	}
	return false
}

func recordMapHasBlock(recordMap map[string]any, id string) bool {
	blocks := notionrecordmap.AsMap(recordMap["block"])
	if blocks == nil {
		return false
	}
	if blocks[id] != nil {
		return true
	}
	return blocks[notionid.CompactID(id)] != nil
}

func arrayStrings(value any) []string {
	out := []string{}
	for _, item := range notionrecordmap.AsSlice(value) {
		if text := notionrecordmap.StringValue(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func ensureRecordMap(recordMap map[string]any, key string) map[string]any {
	current := notionrecordmap.AsMap(recordMap[key])
	if current != nil {
		return current
	}
	current = map[string]any{}
	recordMap[key] = current
	return current
}

func mergeRawRecordMap(target map[string]any, source map[string]any) {
	if source == nil {
		return
	}
	for _, key := range []string{"block", "collection", "collection_view", "notion_user"} {
		sourceMap := notionrecordmap.AsMap(source[key])
		if sourceMap == nil {
			continue
		}
		targetMap := ensureRecordMap(target, key)
		for id, value := range sourceMap {
			targetMap[id] = value
		}
	}
}

func collectionPointerID(value any) string {
	pointer := collectionPointer(value)
	if pointer == nil {
		return ""
	}
	return notionrecordmap.StringValue(pointer["id"])
}

func collectionPointerSpaceID(value any) string {
	pointer := collectionPointer(value)
	if pointer == nil {
		return ""
	}
	return firstNonEmpty(notionrecordmap.StringValue(pointer["spaceId"]), notionrecordmap.StringValue(pointer["space_id"]))
}

func collectionPointer(value any) map[string]any {
	record := notionrecordmap.AsMap(value)
	if record == nil {
		return nil
	}
	return notionrecordmap.AsMap(record["collection_pointer"])
}

func recordMapRootPageID(recordMap map[string]any) string {
	blocks := notionrecordmap.AsMap(recordMap["block"])
	keys := make([]string, 0, len(blocks))
	for key := range blocks {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		block := notionrecordmap.AsMap(notionrecordmap.Unbox(blocks[key]))
		if notionrecordmap.StringValue(block["type"]) == "page" {
			return notionrecordmap.StringValue(block["id"])
		}
	}
	return ""
}

func existingOrNew(parent map[string]any, key string) map[string]any {
	value := notionrecordmap.AsMap(parent[key])
	if value == nil {
		value = map[string]any{}
		parent[key] = value
	}
	return value
}
