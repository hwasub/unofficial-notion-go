package notion

import (
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type collectionGroup struct {
	Key       string
	Label     string
	RowIDs    []string
	Hidden    bool
	Collapsed bool
}

func renderCollectionGroups(out *strings.Builder, groups []collectionGroup, renderRows func([]string)) {
	out.WriteString(`<div class="notion-collection-groups">`)
	for _, group := range groups {
		if group.Hidden || len(group.RowIDs) == 0 {
			continue
		}
		tag := "section"
		if group.Collapsed {
			tag = "details"
		}
		out.WriteString(`<`)
		out.WriteString(tag)
		out.WriteString(` class="notion-collection-group`)
		if group.Collapsed {
			out.WriteString(` notion-collection-group--collapsed`)
		}
		out.WriteString(`">`)
		if group.Collapsed {
			out.WriteString(`<summary>`)
			renderCollectionGroupHeading(out, group)
			out.WriteString(`</summary>`)
		} else {
			out.WriteString(`<h3>`)
			renderCollectionGroupHeading(out, group)
			out.WriteString(`</h3>`)
		}
		renderRows(group.RowIDs)
		out.WriteString(`</`)
		out.WriteString(tag)
		out.WriteString(`>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionGroupHeading(out *strings.Builder, group collectionGroup) {
	out.WriteString(`<span class="notion-collection-group__label">`)
	out.WriteString(html.EscapeString(group.Label))
	out.WriteString(`</span><span class="notion-collection-group__count">`)
	out.WriteString(strconv.Itoa(len(group.RowIDs)))
	out.WriteString(`</span>`)
}

func collectionGroups(query collectionQuery, fallbackRowIDs []string, specs []collectionGroup, input RenderInput) []collectionGroup {
	if len(query.Results) == 0 {
		return nil
	}
	keys := make([]string, 0, len(query.Results))
	for key := range query.Results {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Index non-empty buckets by key and by decoded label so the configured
	// group order (view.Format["collection_groups"], carried in specs) can drive
	// the emitted sequence. Notion shows groups in the user-arranged order, not
	// alphabetically by query-result key.
	idsByKey := make(map[string][]string, len(keys))
	keyByLabel := make(map[string]string, len(keys))
	for _, key := range keys {
		ids := normalizeRowIDs(query.Results[key].BlockIDs)
		if len(ids) == 0 {
			continue
		}
		idsByKey[key] = ids
		if label := collectionGroupLabel(key); label != "" {
			if _, ok := keyByLabel[label]; !ok {
				keyByLabel[label] = key
			}
		}
	}

	groups := make([]collectionGroup, 0, len(keys)+1)
	rendered := map[string]struct{}{}
	usedKey := map[string]struct{}{}
	specByLabel := collectionGroupSpecsByLabel(specs)

	emit := func(key string, spec collectionGroup) {
		ids := idsByKey[key]
		if len(ids) == 0 {
			return
		}
		usedKey[key] = struct{}{}
		for _, id := range ids {
			rendered[id] = struct{}{}
		}
		label := collectionGroupLabel(key)
		groups = append(groups, collectionGroup{
			Key:       key,
			Label:     firstNonEmpty(spec.Label, label),
			RowIDs:    ids,
			Hidden:    spec.Hidden,
			Collapsed: spec.Collapsed,
		})
	}

	// 1) Configured specs, in their declared order.
	for _, spec := range specs {
		if spec.Label == "" {
			continue
		}
		key, ok := keyByLabel[spec.Label]
		if !ok {
			continue
		}
		if _, done := usedKey[key]; done {
			continue
		}
		emit(key, spec)
	}
	// 2) Any remaining buckets, deterministically by sorted key.
	for _, key := range keys {
		if _, done := usedKey[key]; done {
			continue
		}
		emit(key, specByLabel[collectionGroupLabel(key)])
	}
	// 3) Rows not covered by any bucket.
	extra := make([]string, 0)
	for _, id := range normalizeRowIDs(fallbackRowIDs) {
		if _, ok := rendered[id]; !ok {
			extra = append(extra, id)
		}
	}
	if len(extra) > 0 {
		groups = append(groups, collectionGroup{Key: "other", Label: input.t("notion.other", "Other"), RowIDs: extra})
	}
	return groups
}

func collectionGroupSpecsByLabel(specs []collectionGroup) map[string]collectionGroup {
	out := make(map[string]collectionGroup, len(specs))
	for _, spec := range specs {
		if spec.Label != "" {
			out[spec.Label] = spec
		}
	}
	return out
}

func collectionGroupsFromView(rm recordMap, coll collection, view collectionView, rowIDs []string) []collectionGroup {
	property := collectionGroupProperty(view)
	if property == "" {
		return nil
	}
	groupRows := map[string][]string{}
	for _, id := range normalizeRowIDs(rowIDs) {
		row, ok := rm.Block[NormalizeID(id)]
		if !ok {
			continue
		}
		labels := collectionGroupRowLabels(row.Properties[property], coll.Schema[property])
		if len(labels) == 0 {
			labels = []string{"No value"}
		}
		for _, label := range labels {
			label = strings.TrimSpace(label)
			if label == "" {
				label = "No value"
			}
			groupRows[label] = appendUniqueString(groupRows[label], id)
		}
	}
	if len(groupRows) == 0 {
		return nil
	}
	groups := make([]collectionGroup, 0, len(groupRows))
	used := map[string]struct{}{}
	for _, spec := range collectionGroupSpecs(view.Format["collection_groups"], property) {
		ids := groupRows[spec.Label]
		if len(ids) == 0 {
			continue
		}
		used[spec.Label] = struct{}{}
		groups = append(groups, collectionGroup{Key: spec.Key, Label: spec.Label, RowIDs: ids, Hidden: spec.Hidden, Collapsed: spec.Collapsed})
	}
	extraLabels := make([]string, 0, len(groupRows))
	for label := range groupRows {
		if _, ok := used[label]; !ok {
			extraLabels = append(extraLabels, label)
		}
	}
	sort.Strings(extraLabels)
	for _, label := range extraLabels {
		groups = append(groups, collectionGroup{Key: "derived:" + label, Label: label, RowIDs: groupRows[label]})
	}
	if len(groups) == 0 {
		return nil
	}
	return groups
}

func collectionGroupProperty(view collectionView) string {
	for _, value := range []any{
		view.Query2["group_by"],
		view.Query["group_by"],
		view.Format["collection_group_by"],
		view.Format["board_columns_by"],
		view.Format["group_by"],
	} {
		if property := collectionFormatPropertyID(value); property != "" {
			return property
		}
	}
	for _, spec := range collectionGroupSpecs(view.Format["collection_groups"], "") {
		if spec.Key != "" {
			return spec.Key
		}
	}
	return ""
}

func collectionGroupSpecs(value any, fallbackProperty string) []collectionGroup {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	groups := make([]collectionGroup, 0, len(items))
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		property := firstNonEmpty(stringValue(data["property"]), fallbackProperty)
		if fallbackProperty != "" && property != "" && property != fallbackProperty {
			continue
		}
		label := collectionGroupValueLabel(data["value"])
		if label == "" {
			label = "No value"
		}
		groups = append(groups, collectionGroup{
			Key:       property,
			Label:     label,
			Hidden:    collectionGroupHidden(data),
			Collapsed: collectionGroupCollapsed(data),
		})
	}
	return groups
}

func collectionGroupHidden(data map[string]any) bool {
	if boolValue(data["hidden"]) {
		return true
	}
	visible, ok := data["visible"].(bool)
	return ok && !visible
}

func collectionGroupCollapsed(data map[string]any) bool {
	for _, key := range []string{"is_collapsed", "isCollapsed", "collapsed", "is_collapsed_by_default"} {
		if boolValue(data[key]) {
			return true
		}
	}
	return false
}

func collectionGroupValueLabel(value any) string {
	data, ok := value.(map[string]any)
	if !ok {
		return strings.TrimSpace(plainText(value))
	}
	return strings.TrimSpace(stringValue(data["value"]))
}

func collectionGroupRowLabels(value any, schema collectionProperty) []string {
	switch schema.Type {
	case "select", "multi_select", "status":
		return collectionPlainValues(value)
	case "checkbox":
		normalized := strings.ToLower(strings.TrimSpace(plainText(value)))
		if normalized == "yes" || normalized == "true" || normalized == "checked" || normalized == "1" {
			return []string{"Yes"}
		}
		if normalized == "no" || normalized == "false" || normalized == "unchecked" || normalized == "0" {
			return []string{"No"}
		}
		return nil
	default:
		text := strings.TrimSpace(plainText(value))
		if text == "" {
			return nil
		}
		return []string{text}
	}
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func normalizeRowIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		normalized := NormalizeID(id)
		if normalized != "" {
			out = append(out, normalized)
		}
	}
	return dedupeStrings(out)
}

func collectionGroupLabel(key string) string {
	rest := strings.TrimPrefix(strings.TrimSpace(key), "results:")
	if rest == "" || rest == key {
		return "Group"
	}
	parts := strings.Split(rest, ":")
	label := strings.TrimSpace(parts[len(parts)-1])
	if decoded, err := url.QueryUnescape(label); err == nil {
		label = decoded
	}
	if label == "" || strings.EqualFold(label, "uncategorized") {
		return "No value"
	}
	return label
}

func boardColumnLabelAndKey(column collectionBoardColumn) (string, string) {
	valueType := firstNonEmpty(stringValue(column.Value["type"]), "select")
	value := stringValue(column.Value["value"])
	if value == "" {
		return "No status", "results:" + valueType + ":uncategorized"
	}
	return value, "results:" + valueType + ":" + value
}
