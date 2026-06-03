package notion

import (
	"html"
	"math"
	"sort"
	"strconv"
	"strings"
)

func renderCollectionView(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) {
	collectionID := firstNonEmpty(blk.CollectionID, pointerID(blk.Format["collection_pointer"]))
	if collectionID == "" {
		renderCollectionPlaceholder(out, text, input)
		return
	}
	coll := rm.Collection[collectionID]
	views := collectionRenderableViews(rm, collectionID, blk.ViewIDs)
	if len(views) == 0 {
		renderCollectionPlaceholder(out, firstText(richText(coll.Name), text), input)
		return
	}
	title := firstText(richText(coll.Name), firstText(text, input.t("notion.database_view", "Database view")))
	sourceClass := collectionViewSourceClass(blk.Type)
	if len(views) == 1 {
		view := views[0]
		renderCollectionViewSection(out, rm, coll, view, title, false, sourceClass, input)
		return
	}
	out.WriteString(`<section class="notion-collection notion-collection--views`)
	out.WriteString(sourceClass)
	out.WriteString(`">`)
	renderCollectionViewTabs(out, rm, blk, coll, views, title, input)
	out.WriteString(`</section>`)
}

func collectionViewSourceClass(blockType string) string {
	if blockType == "collection_view_page" {
		return " notion-collection--view-page"
	}
	return ""
}

type collectionRenderableView struct {
	ID     string
	View   collectionView
	Query  collectionQuery
	RowIDs []string
}

func collectionRenderableViews(rm recordMap, collectionID string, viewIDs []string) []collectionRenderableView {
	out := make([]collectionRenderableView, 0, len(viewIDs))
	seen := map[string]struct{}{}
	for _, rawID := range viewIDs {
		viewID := NormalizeID(rawID)
		if viewID == "" {
			continue
		}
		if _, ok := seen[viewID]; ok {
			continue
		}
		seen[viewID] = struct{}{}
		view := rm.CollectionView[viewID]
		if view.ID == "" && view.Type == "" {
			continue
		}
		query := rm.CollectionQuery[collectionID][viewID]
		out = append(out, collectionRenderableView{
			ID:     viewID,
			View:   view,
			Query:  query,
			RowIDs: collectionRowIDs(query),
		})
	}
	return out
}

func renderCollectionViewSection(out *strings.Builder, rm recordMap, coll collection, view collectionRenderableView, title string, showViewName bool, sourceClass string, input RenderInput) {
	viewType := sanitizeClassToken(firstNonEmpty(view.View.Type, "list"))
	out.WriteString(`<section class="notion-collection notion-collection--`)
	out.WriteString(html.EscapeString(viewType))
	out.WriteString(sourceClass)
	out.WriteString(`">`)
	viewName := ""
	if showViewName {
		viewName = richText(view.View.Name)
	}
	renderCollectionHead(out, title, viewName)
	renderCollectionViewBody(out, rm, coll, view.View, view.Query, view.RowIDs, input)
	renderCollectionAggregations(out, view.View, coll, view.Query, input)
	out.WriteString(`</section>`)
}

func renderCollectionHead(out *strings.Builder, title string, viewName string) {
	out.WriteString(`<div class="notion-collection__head"><strong>`)
	out.WriteString(title)
	out.WriteString(`</strong>`)
	if viewName != "" {
		out.WriteString(`<span>`)
		out.WriteString(viewName)
		out.WriteString(`</span>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionViewTabs(out *strings.Builder, rm recordMap, blk block, coll collection, views []collectionRenderableView, title string, input RenderInput) {
	base := strings.ReplaceAll(NormalizeID(blk.ID), "-", "")
	if base == "" {
		base = "collection"
	}
	out.WriteString(`<div class="notion-tab-block notion-collection-view-tabs" data-notion-tabs>`)
	out.WriteString(`<div class="notion-tab-list" role="tablist" aria-label="`)
	out.WriteString(html.EscapeString(input.t("notion.database_views", "Database views")))
	out.WriteString(`">`)
	for i, view := range views {
		buttonID, panelID := notionCollectionViewTabIDs(base, i)
		className := "notion-tab-button"
		selected := "false"
		tabIndex := "-1"
		if i == 0 {
			className += " notion-tab-button--active"
			selected = "true"
			tabIndex = "0"
		}
		out.WriteString(`<button type="button" class="`)
		out.WriteString(className)
		out.WriteString(`" role="tab" id="`)
		out.WriteString(html.EscapeString(buttonID))
		out.WriteString(`" aria-selected="`)
		out.WriteString(selected)
		out.WriteString(`" aria-controls="`)
		out.WriteString(html.EscapeString(panelID))
		out.WriteString(`" tabindex="`)
		out.WriteString(tabIndex)
		out.WriteString(`" data-notion-tab-target="`)
		out.WriteString(html.EscapeString(panelID))
		out.WriteString(`">`)
		out.WriteString(`<span class="notion-tab-button__icon notion-tab-button__icon--`)
		out.WriteString(html.EscapeString(sanitizeClassToken(firstNonEmpty(view.View.Type, "view"))))
		out.WriteString(`" aria-hidden="true"></span><span class="notion-tab-button__label">`)
		out.WriteString(html.EscapeString(collectionViewLabel(view.View, i)))
		out.WriteString(`</span></button>`)
	}
	out.WriteString(`</div>`)
	renderCollectionHead(out, title, "")
	for i, view := range views {
		buttonID, panelID := notionCollectionViewTabIDs(base, i)
		viewType := sanitizeClassToken(firstNonEmpty(view.View.Type, "list"))
		out.WriteString(`<section class="notion-tab-panel notion-collection-view-panel notion-collection-view-panel--`)
		out.WriteString(html.EscapeString(viewType))
		out.WriteString(`" role="tabpanel" tabindex="0" id="`)
		out.WriteString(html.EscapeString(panelID))
		out.WriteString(`" aria-labelledby="`)
		out.WriteString(html.EscapeString(buttonID))
		out.WriteString(`" data-notion-tab-panel`)
		if i > 0 {
			out.WriteString(` hidden`)
		}
		out.WriteString(`>`)
		renderCollectionViewBody(out, rm, coll, view.View, view.Query, view.RowIDs, input)
		renderCollectionAggregations(out, view.View, coll, view.Query, input)
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
}

func notionCollectionViewTabIDs(base string, index int) (string, string) {
	suffix := strconv.Itoa(index)
	return "notion-collection-view-" + base + "-tab-" + suffix, "notion-collection-view-" + base + "-panel-" + suffix
}

func collectionViewLabel(view collectionView, index int) string {
	if label := strings.TrimSpace(plainText(view.Name)); label != "" {
		return label
	}
	switch strings.ToLower(strings.TrimSpace(view.Type)) {
	case "table":
		return "Table"
	case "list":
		return "List"
	case "gallery":
		return "Gallery"
	case "board":
		return "Board"
	case "calendar":
		return "Calendar"
	case "timeline":
		return "Timeline"
	default:
		return "View " + strconv.Itoa(index+1)
	}
}

func renderCollectionViewBody(out *strings.Builder, rm recordMap, coll collection, view collectionView, query collectionQuery, rowIDs []string, input RenderInput) {
	if len(rowIDs) == 0 {
		renderCollectionPlaceholder(out, firstText(richText(coll.Name), input.t("notion.database_view", "Database view")), input)
		return
	}
	viewType := sanitizeClassToken(firstNonEmpty(view.Type, "list"))
	properties := visibleCollectionProperties(view, coll)
	groupProperty := collectionGroupProperty(view)
	groupSpecs := collectionGroupSpecs(view.Format["collection_groups"], groupProperty)
	groups := collectionGroups(query, rowIDs, groupSpecs, input)
	if len(groups) == 0 {
		groups = collectionGroupsFromView(rm, coll, view, rowIDs)
	}
	switch view.Type {
	case "table":
		tableProperties := visibleCollectionPropertySpecs(view, coll)
		if len(groups) > 0 {
			renderCollectionGroupedTable(out, rm, coll, view, groups, tableProperties, input)
		} else {
			renderCollectionTable(out, rm, coll, view, rowIDs, tableProperties, input)
		}
	case "list":
		if len(groups) > 0 {
			renderCollectionGroupedList(out, rm, coll, groups, properties, input)
		} else {
			renderCollectionList(out, rm, coll, rowIDs, properties, input)
		}
	case "board":
		renderCollectionBoard(out, rm, coll, query, rowIDs, properties, input)
	case "calendar":
		renderCollectionCalendar(out, rm, coll, view, rowIDs, properties, input)
	case "timeline":
		renderCollectionTimeline(out, rm, coll, view, rowIDs, properties, input)
	default:
		if len(groups) > 0 {
			renderCollectionGroupedCards(out, rm, coll, groups, properties, viewType, input)
		} else {
			renderCollectionCards(out, rm, coll, rowIDs, properties, viewType, input)
		}
	}
}

func renderCollectionPlaceholder(out *strings.Builder, text string, input RenderInput) {
	out.WriteString(`<div class="notion-collection-placeholder">`)
	out.WriteString(firstText(text, input.t("notion.database_view", "Database view")))
	out.WriteString(`</div>`)
}

func renderCollectionTable(out *strings.Builder, rm recordMap, coll collection, view collectionView, rowIDs []string, properties []collectionViewProperty, input RenderInput) {
	className := "notion-collection-table"
	if boolValue(view.Format["table_wrap"]) {
		className += " notion-collection-table--wrap"
	}
	out.WriteString(`<div class="notion-table-wrap"><table class="`)
	out.WriteString(className)
	out.WriteString(`"><colgroup>`)
	for _, spec := range properties {
		out.WriteString(`<col style="width:`)
		out.WriteString(strconv.Itoa(spec.Width))
		out.WriteString(`px">`)
	}
	out.WriteString(`</colgroup><thead><tr>`)
	for _, spec := range properties {
		out.WriteString(`<th scope="col">`)
		renderCollectionPropertyHead(out, coll, spec.Property)
		out.WriteString(`</th>`)
	}
	out.WriteString(`</tr></thead><tbody>`)
	for _, id := range rowIDs {
		row, ok := rm.Block[NormalizeID(id)]
		if !ok {
			continue
		}
		out.WriteString(`<tr>`)
		for _, spec := range properties {
			out.WriteString(`<td>`)
			if cell := collectionPropertyHTML(rm, row, coll, spec.Property, input); strings.TrimSpace(cell) != "" {
				out.WriteString(cell)
			} else {
				out.WriteString(`&nbsp;`)
			}
			out.WriteString(`</td>`)
		}
		out.WriteString(`</tr>`)
	}
	out.WriteString(`</tbody></table></div>`)
}

func renderCollectionPropertyHead(out *strings.Builder, coll collection, property string) {
	schema := collectionProperty{}
	if current, ok := coll.Schema[property]; ok {
		schema = current
	}
	schemaType := firstNonEmpty(schema.Type, property)
	out.WriteString(`<span class="notion-property-head"><span class="notion-property-head__icon notion-property-head__icon--`)
	out.WriteString(html.EscapeString(sanitizeClassToken(schemaType)))
	out.WriteString(`" aria-hidden="true"></span><span class="notion-property-head__label">`)
	out.WriteString(html.EscapeString(collectionPropertyName(coll, property)))
	out.WriteString(`</span></span>`)
}

func renderCollectionGroupedTable(out *strings.Builder, rm recordMap, coll collection, view collectionView, groups []collectionGroup, properties []collectionViewProperty, input RenderInput) {
	renderCollectionGroups(out, groups, func(rowIDs []string) {
		renderCollectionTable(out, rm, coll, view, rowIDs, properties, input)
	})
}

func renderCollectionCards(out *strings.Builder, rm recordMap, coll collection, rowIDs []string, properties []string, viewType string, input RenderInput) {
	out.WriteString(`<div class="notion-collection__cards">`)
	for _, id := range rowIDs {
		row, ok := rm.Block[NormalizeID(id)]
		if !ok {
			continue
		}
		renderCollectionCard(out, rm, row, coll, properties, viewType, input)
	}
	out.WriteString(`</div>`)
}

func renderCollectionGroupedCards(out *strings.Builder, rm recordMap, coll collection, groups []collectionGroup, properties []string, viewType string, input RenderInput) {
	renderCollectionGroups(out, groups, func(rowIDs []string) {
		renderCollectionCards(out, rm, coll, rowIDs, properties, viewType, input)
	})
}

func renderCollectionCard(out *strings.Builder, rm recordMap, row block, coll collection, properties []string, viewType string, input RenderInput) {
	out.WriteString(`<article class="notion-collection-card notion-collection-card--`)
	out.WriteString(html.EscapeString(viewType))
	out.WriteString(`">`)
	if cover := collectionCoverHTML(rm, row, input); cover != "" {
		out.WriteString(cover)
	}
	title := collectionPropertyHTML(rm, row, coll, "title", input)
	out.WriteString(`<strong>`)
	out.WriteString(firstText(title, input.t("notion.untitled", "Untitled")))
	out.WriteString(`</strong>`)
	for _, property := range properties {
		if property == "title" {
			continue
		}
		value := collectionPropertyHTML(rm, row, coll, property, input)
		if value == "" {
			continue
		}
		out.WriteString(`<dl><dt>`)
		out.WriteString(html.EscapeString(collectionPropertyName(coll, property)))
		out.WriteString(`</dt><dd>`)
		out.WriteString(value)
		out.WriteString(`</dd></dl>`)
	}
	out.WriteString(`</article>`)
}

func renderCollectionBoard(out *strings.Builder, rm recordMap, coll collection, query collectionQuery, fallbackRowIDs []string, properties []string, input RenderInput) {
	if len(query.BoardColumns.Results) == 0 {
		renderCollectionCards(out, rm, coll, fallbackRowIDs, properties, "board", input)
		return
	}
	out.WriteString(`<div class="notion-collection-board">`)
	renderedRows := map[string]struct{}{}
	for _, column := range query.BoardColumns.Results {
		label, key := boardColumnLabelAndKey(column)
		ids := query.Results[key].BlockIDs
		if collectionBoardColumnHidden(column) {
			for _, id := range ids {
				renderedRows[NormalizeID(id)] = struct{}{}
			}
			continue
		}
		out.WriteString(`<section class="notion-collection-board__lane"><h3>`)
		out.WriteString(html.EscapeString(label))
		out.WriteString(`</h3><div class="notion-collection-board__cards">`)
		for _, id := range ids {
			row, ok := rm.Block[NormalizeID(id)]
			if !ok {
				continue
			}
			renderCollectionCard(out, rm, row, coll, properties, "board", input)
			renderedRows[NormalizeID(id)] = struct{}{}
		}
		out.WriteString(`</div></section>`)
	}
	extraIDs := make([]string, 0)
	for _, id := range fallbackRowIDs {
		normalized := NormalizeID(id)
		if _, ok := renderedRows[normalized]; !ok {
			extraIDs = append(extraIDs, normalized)
		}
	}
	if len(extraIDs) > 0 {
		out.WriteString(`<section class="notion-collection-board__lane"><h3>`)
		out.WriteString(html.EscapeString(input.t("notion.other", "Other")))
		out.WriteString(`</h3><div class="notion-collection-board__cards">`)
		for _, id := range extraIDs {
			row, ok := rm.Block[NormalizeID(id)]
			if !ok {
				continue
			}
			renderCollectionCard(out, rm, row, coll, properties, "board", input)
		}
		out.WriteString(`</div></section>`)
	}
	out.WriteString(`</div>`)
}

func collectionBoardColumnHidden(column collectionBoardColumn) bool {
	if column.Hidden {
		return true
	}
	return column.Visible != nil && !*column.Visible
}

func renderCollectionCalendar(out *strings.Builder, rm recordMap, coll collection, view collectionView, rowIDs []string, properties []string, input RenderInput) {
	dateProperty := collectionDateProperty(view, coll, properties, "calendar")
	buckets, unscheduled := collectionDateBuckets(rm, coll, rowIDs, dateProperty)
	if len(buckets) == 0 && len(unscheduled) == 0 {
		renderCollectionCards(out, rm, coll, rowIDs, properties, "calendar", input)
		return
	}
	out.WriteString(`<div class="notion-collection-calendar">`)
	for _, bucket := range buckets {
		out.WriteString(`<section class="notion-collection-calendar__day"><h3>`)
		if bucket.Key != "" {
			out.WriteString(`<time datetime="`)
			out.WriteString(html.EscapeString(bucket.Key))
			out.WriteString(`">`)
			out.WriteString(html.EscapeString(bucket.Label))
			out.WriteString(`</time>`)
		} else {
			out.WriteString(html.EscapeString(bucket.Label))
		}
		out.WriteString(`</h3><div class="notion-collection-calendar__cards">`)
		for _, id := range bucket.RowIDs {
			if row, ok := rm.Block[NormalizeID(id)]; ok {
				renderCollectionCard(out, rm, row, coll, properties, "calendar", input)
			}
		}
		out.WriteString(`</div></section>`)
	}
	if len(unscheduled) > 0 {
		out.WriteString(`<section class="notion-collection-calendar__day notion-collection-calendar__day--unscheduled"><h3>`)
		out.WriteString(html.EscapeString(input.t("notion.no_date", "No date")))
		out.WriteString(`</h3><div class="notion-collection-calendar__cards">`)
		for _, id := range unscheduled {
			if row, ok := rm.Block[NormalizeID(id)]; ok {
				renderCollectionCard(out, rm, row, coll, properties, "calendar", input)
			}
		}
		out.WriteString(`</div></section>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionTimeline(out *strings.Builder, rm recordMap, coll collection, view collectionView, rowIDs []string, properties []string, input RenderInput) {
	dateProperty := collectionDateProperty(view, coll, properties, "timeline")
	entries, unscheduled := collectionTimelineEntries(rm, coll, rowIDs, dateProperty)
	if len(entries) == 0 && len(unscheduled) == 0 {
		renderCollectionCards(out, rm, coll, rowIDs, properties, "timeline", input)
		return
	}
	if len(entries) == 0 {
		renderCollectionTimelineList(out, rm, coll, entries, unscheduled, properties, input)
		return
	}
	minDate, maxDate := entries[0].Start, entries[0].End
	for _, entry := range entries[1:] {
		if entry.Start.Before(minDate) {
			minDate = entry.Start
		}
		if entry.End.After(maxDate) {
			maxDate = entry.End
		}
	}
	dayCount := daysBetween(minDate, maxDate) + 1
	if dayCount <= 0 || dayCount > 120 {
		renderCollectionTimelineList(out, rm, coll, entries, unscheduled, properties, input)
		return
	}
	out.WriteString(`<div class="notion-collection-timeline">`)
	out.WriteString(`<div class="notion-collection-timeline__axis" style="--notion-timeline-days:`)
	out.WriteString(strconv.Itoa(dayCount))
	out.WriteString(`" aria-hidden="true">`)
	for day := 0; day < dayCount; day++ {
		current := minDate.AddDate(0, 0, day)
		out.WriteString(`<span>`)
		out.WriteString(html.EscapeString(current.Format("Jan 2")))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</div>`)
	for _, entry := range entries {
		row, ok := rm.Block[entry.RowID]
		if !ok {
			continue
		}
		start := daysBetween(minDate, entry.Start) + 1
		span := daysBetween(entry.Start, entry.End) + 1
		if start < 1 {
			start = 1
		}
		if span < 1 {
			span = 1
		}
		if start+span-1 > dayCount {
			span = dayCount - start + 1
		}
		out.WriteString(`<article class="notion-collection-timeline__item" style="--notion-timeline-days:`)
		out.WriteString(strconv.Itoa(dayCount))
		out.WriteString(`;--notion-timeline-start:`)
		out.WriteString(strconv.Itoa(start))
		out.WriteString(`;--notion-timeline-span:`)
		out.WriteString(strconv.Itoa(span))
		out.WriteString(`"><div class="notion-collection-timeline__date"><time datetime="`)
		out.WriteString(html.EscapeString(entry.StartKey))
		out.WriteString(`">`)
		out.WriteString(html.EscapeString(entry.Label))
		out.WriteString(`</time></div><div class="notion-collection-timeline__bar">`)
		renderCollectionCard(out, rm, row, coll, properties, "timeline", input)
		out.WriteString(`</div></article>`)
	}
	if len(unscheduled) > 0 {
		out.WriteString(`<section class="notion-collection-timeline__unscheduled"><h3>`)
		out.WriteString(html.EscapeString(input.t("notion.no_date", "No date")))
		out.WriteString(`</h3>`)
		for _, id := range unscheduled {
			if row, ok := rm.Block[NormalizeID(id)]; ok {
				renderCollectionCard(out, rm, row, coll, properties, "timeline", input)
			}
		}
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionTimelineList(out *strings.Builder, rm recordMap, coll collection, entries []collectionTimelineEntry, unscheduled []string, properties []string, input RenderInput) {
	out.WriteString(`<div class="notion-collection-timeline">`)
	for _, entry := range entries {
		row, ok := rm.Block[entry.RowID]
		if !ok {
			continue
		}
		out.WriteString(`<article class="notion-collection-timeline__item notion-collection-timeline__item--list"><div class="notion-collection-timeline__date"><time datetime="`)
		out.WriteString(html.EscapeString(entry.StartKey))
		out.WriteString(`">`)
		out.WriteString(html.EscapeString(entry.Label))
		out.WriteString(`</time></div><div class="notion-collection-timeline__bar">`)
		renderCollectionCard(out, rm, row, coll, properties, "timeline", input)
		out.WriteString(`</div></article>`)
	}
	if len(unscheduled) > 0 {
		out.WriteString(`<section class="notion-collection-timeline__unscheduled"><h3>`)
		out.WriteString(html.EscapeString(input.t("notion.no_date", "No date")))
		out.WriteString(`</h3>`)
		for _, id := range unscheduled {
			if row, ok := rm.Block[NormalizeID(id)]; ok {
				renderCollectionCard(out, rm, row, coll, properties, "timeline", input)
			}
		}
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionList(out *strings.Builder, rm recordMap, coll collection, rowIDs []string, properties []string, input RenderInput) {
	out.WriteString(`<div class="notion-collection-list">`)
	for _, id := range rowIDs {
		row, ok := rm.Block[NormalizeID(id)]
		if !ok {
			continue
		}
		out.WriteString(`<article class="notion-collection-list__item">`)
		title := collectionPropertyHTML(rm, row, coll, "title", input)
		out.WriteString(`<strong>`)
		out.WriteString(firstText(title, input.t("notion.untitled", "Untitled")))
		out.WriteString(`</strong>`)
		renderCollectionInlineProperties(out, rm, row, coll, properties, input)
		out.WriteString(`</article>`)
	}
	out.WriteString(`</div>`)
}

func renderCollectionGroupedList(out *strings.Builder, rm recordMap, coll collection, groups []collectionGroup, properties []string, input RenderInput) {
	renderCollectionGroups(out, groups, func(rowIDs []string) {
		renderCollectionList(out, rm, coll, rowIDs, properties, input)
	})
}

func renderCollectionAggregations(out *strings.Builder, view collectionView, coll collection, query collectionQuery, input RenderInput) {
	results := query.CollectionGroupResults.AggregationResults
	if len(results) == 0 {
		return
	}
	configs := collectionAggregationConfigs(view)
	var b strings.Builder
	for i, result := range results {
		config := collectionAggregationConfig{}
		if i < len(configs) {
			config = configs[i]
		}
		value := collectionAggregationValue(result, config, coll)
		if value == "" {
			continue
		}
		label := collectionAggregationLabel(config, coll)
		if label == "" {
			label = input.t("notion.result", "Result")
		}
		b.WriteString(`<span><strong>`)
		b.WriteString(html.EscapeString(label))
		b.WriteString(`</strong> `)
		b.WriteString(html.EscapeString(value))
		b.WriteString(`</span>`)
	}
	if b.Len() == 0 {
		return
	}
	out.WriteString(`<div class="notion-collection-aggregations">`)
	out.WriteString(b.String())
	out.WriteString(`</div>`)
}

type collectionAggregationConfig struct {
	Property   string
	Aggregator string
}

func collectionAggregationConfigs(view collectionView) []collectionAggregationConfig {
	out := make([]collectionAggregationConfig, 0)
	for _, source := range []map[string]any{view.Query2, view.Query, view.Format} {
		if len(source) == 0 {
			continue
		}
		out = appendCollectionAggregationConfigs(out, source["aggregations"])
		out = appendCollectionAggregationConfigs(out, source["aggregate"])
	}
	return out
}

func appendCollectionAggregationConfigs(out []collectionAggregationConfig, value any) []collectionAggregationConfig {
	items, ok := value.([]any)
	if !ok {
		return out
	}
	for _, item := range items {
		data, ok := item.(map[string]any)
		if !ok {
			continue
		}
		property := firstNonEmpty(stringValue(data["property"]), stringValue(data["property_id"]))
		aggregator := firstNonEmpty(stringValue(data["aggregator"]), stringValue(data["aggregation_type"]), stringValue(data["id"]))
		if property == "" && aggregator == "" {
			continue
		}
		out = append(out, collectionAggregationConfig{Property: property, Aggregator: aggregator})
	}
	return out
}

func collectionAggregationLabel(config collectionAggregationConfig, coll collection) string {
	aggregator := collectionAggregatorLabel(config.Aggregator)
	property := strings.TrimSpace(config.Property)
	if property == "" {
		return aggregator
	}
	propertyName := collectionPropertyName(coll, property)
	if aggregator == "" {
		return propertyName
	}
	return propertyName + " " + aggregator
}

func collectionAggregatorLabel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "count":
		return "Count"
	case "count_values":
		return "Count values"
	case "count_unique", "count_unique_values", "unique":
		return "Unique"
	case "empty", "count_empty":
		return "Empty"
	case "not_empty", "count_not_empty":
		return "Not empty"
	case "percent_empty":
		return "% empty"
	case "percent_not_empty":
		return "% not empty"
	case "sum":
		return "Sum"
	case "average", "mean":
		return "Average"
	case "median":
		return "Median"
	case "min", "minimum":
		return "Min"
	case "max", "maximum":
		return "Max"
	case "range":
		return "Range"
	case "earliest_date":
		return "Earliest"
	case "latest_date":
		return "Latest"
	case "checked":
		return "Checked"
	case "unchecked":
		return "Unchecked"
	case "percent_checked":
		return "% checked"
	case "percent_unchecked":
		return "% unchecked"
	default:
		return ""
	}
}

func collectionAggregationValue(result collectionAggregationResult, config collectionAggregationConfig, coll collection) string {
	return collectionAggregationValueAny(result.Value, result.Type, config, coll)
}

func collectionAggregationValueAny(value any, resultType string, config collectionAggregationConfig, coll collection) string {
	if value == nil {
		return ""
	}
	if nested, ok := value.(map[string]any); ok {
		if nestedValue, exists := nested["value"]; exists {
			return collectionAggregationValueAny(nestedValue, stringValue(nested["type"]), config, coll)
		}
		return ""
	}
	if number, ok := numberValue(value); ok && !math.IsNaN(number) && !math.IsInf(number, 0) {
		return formatCollectionAggregationNumber(number, resultType, config, coll)
	}
	if boolValue, ok := value.(bool); ok {
		if boolValue {
			return "Yes"
		}
		return "No"
	}
	if text := plainText(value); strings.TrimSpace(text) != "" {
		return strings.TrimSpace(text)
	}
	text := stringValue(value)
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func formatCollectionAggregationNumber(value float64, resultType string, config collectionAggregationConfig, coll collection) string {
	aggregator := strings.ToLower(strings.TrimSpace(config.Aggregator))
	if strings.HasPrefix(aggregator, "percent_") || strings.EqualFold(resultType, "percent") {
		return formatCollectionNumber(value, "percent")
	}
	if property, ok := coll.Schema[config.Property]; ok {
		switch property.Type {
		case "number", "formula", "rollup":
			if format := strings.TrimSpace(property.NumberFormat); format != "" && collectionAggregatorUsesPropertyNumberFormat(aggregator) {
				return formatCollectionNumber(value, format)
			}
		}
	}
	return formatFloat(value)
}

func collectionAggregatorUsesPropertyNumberFormat(aggregator string) bool {
	switch aggregator {
	case "sum", "average", "mean", "median", "min", "minimum", "max", "maximum", "range":
		return true
	default:
		return false
	}
}

func renderCollectionInlineProperties(out *strings.Builder, rm recordMap, row block, coll collection, properties []string, input RenderInput) {
	wrote := false
	for _, property := range properties {
		if property == "title" {
			continue
		}
		value := collectionPropertyHTML(rm, row, coll, property, input)
		if value == "" {
			continue
		}
		if !wrote {
			out.WriteString(`<dl>`)
			wrote = true
		}
		out.WriteString(`<dt>`)
		out.WriteString(html.EscapeString(collectionPropertyName(coll, property)))
		out.WriteString(`</dt><dd>`)
		out.WriteString(value)
		out.WriteString(`</dd>`)
	}
	if wrote {
		out.WriteString(`</dl>`)
	}
}

func collectionRowIDs(query collectionQuery) []string {
	out := append([]string{}, query.CollectionGroupResults.BlockIDs...)
	if len(out) == 0 && len(query.Results) > 0 {
		keys := make([]string, 0, len(query.Results))
		for key := range query.Results {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out = append(out, query.Results[key].BlockIDs...)
		}
	}
	return dedupeStrings(out)
}
