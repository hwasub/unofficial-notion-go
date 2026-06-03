package notion

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type collectionDateBucket struct {
	Key    string
	Label  string
	RowIDs []string
}

type collectionTimelineEntry struct {
	RowID    string
	Label    string
	StartKey string
	EndKey   string
	Start    time.Time
	End      time.Time
}

func collectionDateBuckets(rm recordMap, coll collection, rowIDs []string, dateProperty string) ([]collectionDateBucket, []string) {
	if dateProperty == "" {
		return nil, rowIDs
	}
	byKey := map[string]*collectionDateBucket{}
	unscheduled := make([]string, 0)
	for _, rawID := range rowIDs {
		id := NormalizeID(rawID)
		row, ok := rm.Block[id]
		if !ok {
			continue
		}
		label, key := collectionDateLabelAndKey(row.Properties[dateProperty])
		if key == "" {
			unscheduled = append(unscheduled, id)
			continue
		}
		bucket := byKey[key]
		if bucket == nil {
			bucket = &collectionDateBucket{Key: key, Label: firstNonEmpty(label, key)}
			byKey[key] = bucket
		}
		bucket.RowIDs = append(bucket.RowIDs, id)
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]collectionDateBucket, 0, len(keys))
	for _, key := range keys {
		out = append(out, *byKey[key])
	}
	return out, unscheduled
}

func collectionTimelineEntries(rm recordMap, coll collection, rowIDs []string, dateProperty string) ([]collectionTimelineEntry, []string) {
	if dateProperty == "" {
		return nil, rowIDs
	}
	entries := make([]collectionTimelineEntry, 0, len(rowIDs))
	unscheduled := make([]string, 0)
	for _, rawID := range rowIDs {
		id := NormalizeID(rawID)
		row, ok := rm.Block[id]
		if !ok {
			continue
		}
		entry, ok := collectionTimelineEntryForRow(id, row.Properties[dateProperty])
		if !ok {
			unscheduled = append(unscheduled, id)
			continue
		}
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].Start.Equal(entries[j].Start) {
			return entries[i].Start.Before(entries[j].Start)
		}
		if !entries[i].End.Equal(entries[j].End) {
			return entries[i].End.Before(entries[j].End)
		}
		return entries[i].RowID < entries[j].RowID
	})
	return entries, unscheduled
}

func collectionTimelineEntryForRow(rowID string, value any) (collectionTimelineEntry, bool) {
	label, startKey, endKey := collectionDateRangeLabelAndKeys(value)
	if startKey == "" {
		return collectionTimelineEntry{}, false
	}
	start, ok := parseDateKey(startKey)
	if !ok {
		return collectionTimelineEntry{}, false
	}
	end := start
	if endKey != "" {
		if parsedEnd, ok := parseDateKey(endKey); ok {
			end = parsedEnd
		}
	}
	if end.Before(start) {
		end = start
	}
	return collectionTimelineEntry{
		RowID:    NormalizeID(rowID),
		Label:    firstNonEmpty(label, startKey),
		StartKey: startKey,
		EndKey:   firstNonEmpty(endKey, startKey),
		Start:    start,
		End:      end,
	}, true
}

func collectionDateProperty(view collectionView, coll collection, properties []string, viewType string) string {
	for _, key := range []string{
		viewType + "_by",
		viewType + "_date",
		viewType + "_date_property",
		viewType + "_property",
		"date_by",
		"date_property",
		"property",
	} {
		if property := collectionFormatPropertyID(view.Format[key]); collectionSchemaIsDate(coll, property) {
			return property
		}
	}
	for _, property := range properties {
		if collectionSchemaIsDate(coll, property) {
			return property
		}
	}
	keys := make([]string, 0, len(coll.Schema))
	for property := range coll.Schema {
		keys = append(keys, property)
	}
	sort.Strings(keys)
	for _, property := range keys {
		if collectionSchemaIsDate(coll, property) {
			return property
		}
	}
	return ""
}

func collectionFormatPropertyID(value any) string {
	if value == nil {
		return ""
	}
	if property, ok := value.(string); ok {
		return strings.TrimSpace(property)
	}
	if data, ok := value.(map[string]any); ok {
		return firstNonEmpty(stringValue(data["property"]), stringValue(data["property_id"]), stringValue(data["id"]))
	}
	return ""
}

func collectionSchemaIsDate(coll collection, property string) bool {
	if property == "" {
		return false
	}
	schema, ok := coll.Schema[property]
	if !ok {
		return false
	}
	switch schema.Type {
	case "date", "created_time", "last_edited_time":
		return true
	default:
		return false
	}
}

func collectionDateLabelAndKey(value any) (string, string) {
	if label, start := richTextDateMention(value); label != "" {
		return label, dateKey(start)
	}
	if label, start, _ := formatDateRangeMention(value); label != "" {
		return label, dateKey(start)
	}
	text := plainText(value)
	if text == "" {
		text = strings.TrimSpace(fmt.Sprint(value))
	}
	text = strings.TrimSpace(text)
	if text == "" || text == "<nil>" {
		return "", ""
	}
	return text, dateKey(text)
}

func collectionDateRangeLabelAndKeys(value any) (string, string, string) {
	if label, start, end := richTextDateRangeMention(value); label != "" {
		return label, dateKey(start), dateKey(firstNonEmpty(end, start))
	}
	if label, start, end := formatDateRangeMention(value); label != "" {
		return label, dateKey(start), dateKey(firstNonEmpty(end, start))
	}
	text := plainText(value)
	if text == "" {
		text = strings.TrimSpace(fmt.Sprint(value))
	}
	text = strings.TrimSpace(text)
	if text == "" || text == "<nil>" {
		return "", "", ""
	}
	key := dateKey(text)
	return text, key, key
}

func richTextDateMention(value any) (string, string) {
	label, start, _ := richTextDateRangeMention(value)
	return label, start
}

func richTextDateRangeMention(value any) (string, string, string) {
	raw, ok := value.([]any)
	if !ok {
		return "", "", ""
	}
	for _, part := range raw {
		arr, ok := part.([]any)
		if !ok || len(arr) < 2 {
			continue
		}
		decorations, ok := arr[1].([]any)
		if !ok {
			continue
		}
		for _, deco := range decorations {
			item, ok := deco.([]any)
			if !ok || len(item) < 2 {
				continue
			}
			kind, _ := item[0].(string)
			if kind != "d" {
				continue
			}
			return formatDateRangeMention(item[1])
		}
	}
	return "", "", ""
}

func formatDateMention(value any) (string, string) {
	label, start, _ := formatDateRangeMention(value)
	return label, start
}

func formatDateRangeMention(value any) (string, string, string) {
	start, end, startTime, endTime, ok := dateRangeFields(value)
	if !ok || start == "" {
		return "", "", ""
	}
	startLabel, startDatetime := collectionDateTextLabel(start)
	if startLabel == "" {
		return "", "", ""
	}
	endLabel := ""
	endDatetime := ""
	if end != "" {
		endLabel, endDatetime = collectionDateTextLabel(end)
	}
	label := startLabel
	if startTime != "" {
		label += " " + collectionClockLabel(startTime)
	}
	if endLabel != "" && endLabel != startLabel {
		usedEndClock := false
		if sameDay, endClock := sameDayEndClockLabel(start, end); sameDay && endClock != "" {
			label += " → " + endClock
			usedEndClock = true
		} else {
			label += " → " + endLabel
		}
		if endTime != "" && !usedEndClock {
			label += " " + collectionClockLabel(endTime)
		}
	} else if endTime != "" {
		label += " → " + collectionClockLabel(endTime)
	}
	return label, startDatetime, endDatetime
}

func dateRangeFields(value any) (string, string, string, string, bool) {
	data, ok := value.(map[string]any)
	if !ok {
		return "", "", "", "", false
	}
	if payload, payloadType, ok := computedValuePayload(data); ok && strings.EqualFold(payloadType, "date") {
		if payloadData, ok := payload.(map[string]any); ok {
			data = payloadData
		}
	}
	start := cleanDateField(firstNonEmpty(stringValue(data["start_date"]), stringValue(data["start"])))
	if start == "" {
		return "", "", "", "", false
	}
	end := cleanDateField(firstNonEmpty(stringValue(data["end_date"]), stringValue(data["end"])))
	startTime := cleanDateField(firstNonEmpty(stringValue(data["start_time"]), stringValue(data["startTime"])))
	endTime := cleanDateField(firstNonEmpty(stringValue(data["end_time"]), stringValue(data["endTime"])))
	return start, end, startTime, endTime, true
}

func cleanDateField(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return ""
	}
	return value
}

func dateKey(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 && isISODatePrefix(value[:10]) {
		return value[:10]
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Format("2006-01-02")
	}
	for _, field := range strings.Fields(value) {
		field = strings.Trim(field, ".,;()[]{}")
		if len(field) >= 10 && isISODatePrefix(field[:10]) {
			return field[:10]
		}
	}
	return ""
}

func parseDateKey(value string) (time.Time, bool) {
	parsed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	return parsed, err == nil
}

func daysBetween(start time.Time, end time.Time) int {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	return int(end.Sub(start).Hours() / 24)
}

func isISODatePrefix(value string) bool {
	if len(value) != 10 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 4, 7:
			if ch != '-' {
				return false
			}
		default:
			if ch < '0' || ch > '9' {
				return false
			}
		}
	}
	return true
}
