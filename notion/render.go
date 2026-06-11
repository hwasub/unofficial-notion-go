package notion

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// RenderInput is the complete set of inputs RenderPage needs to produce HTML for
// one Notion page. Everything the renderer trusts about asset and link
// destinations comes from here, keeping URL safety decisions with the caller.
type RenderInput struct {
	RecordMap json.RawMessage   // raw Notion record map containing the page and its blocks
	PageID    string            // ID of the page within RecordMap to render as root
	PagePaths map[string]string // page ID -> URL path, used to link subpages and mentions
	AssetURLs map[string]string // asset key -> resolved URL for images, files, and media
	Warnings  []RenderWarning   // advisories rendered as a banner ahead of the content
	// UnsafeRenderNotionSignedURLs emits direct Notion-hosted signed asset URLs
	// from the record map/source when AssetURLs has no caller-controlled URL.
	// This is only for local debugging and must not be enabled in production.
	UnsafeRenderNotionSignedURLs bool
	ResourceSlug                 string    // first URL path segment for emitted page links (e.g. "/<slug>/<page>"); empty disables those links
	T                            Localizer // optional localizer for UI strings; nil uses English fallbacks
}

// Localizer returns localized UI text for a key, falling back to a built-in
// English string. RenderPage callers may supply one bound to the page
// owner's locale; a nil Localizer (tests/other callers) uses the fallbacks.
//
// Localizer must return PLAIN TEXT; the renderer escapes the result before
// embedding it in HTML.
//
// The renderer currently requests these keys:
//
//	Navigation:    nav.breadcrumb
//	Blocks/UI:     notion.copy, notion.copied, notion.copy_code,
//	               notion.download_file, notion.embed, notion.pdf,
//	               notion.tweet, notion.tabs, notion.toggle_details,
//	               notion.untitled
//	Databases:     notion.database_view, notion.database_views,
//	               notion.no_date, notion.other, notion.result
//	Warnings:      notion.render_warning_title, notion.warn_fetch,
//	               notion.warn_asset_download, notion.warn_assets_truncated,
//	               notion.warn_blocks_truncated
//
// New keys may be added in minor releases. Implementations must return the
// supplied fallback for keys they do not recognize.
type Localizer func(key, fallback string) string

func (in RenderInput) t(key, fallback string) string {
	if in.T == nil {
		return fallback
	}
	return in.T(key, fallback)
}

type recordMap struct {
	Block            map[string]block                      `json:"block"`
	Collection       map[string]collection                 `json:"collection"`
	CollectionView   map[string]collectionView             `json:"collection_view"`
	CollectionQuery  map[string]map[string]collectionQuery `json:"collection_query"`
	Automation       map[string]automation                 `json:"automation"`
	AutomationAction map[string]automationAction           `json:"automation_action"`
	CustomEmojis     map[string]string                     `json:"custom_emojis"`
	SignedURLs       map[string]string                     `json:"signed_urls"`

	// budget caps the total number of block renders for one RenderPage call.
	// recordMap is passed by value, so the pointer is shared by every copy.
	// The depth guard alone bounds recursion depth but not fan-out: a hostile
	// record map with cyclic or repeated content references can otherwise blow
	// up exponentially (~b^32 visits) from a few hundred bytes of JSON. nil
	// (e.g. in direct test constructions) means unlimited.
	budget *renderBudget
}

type renderBudget struct {
	remaining int
}

// spendRenderBudget reports whether one more block may be rendered, consuming
// a budget slot when it is. Rendering stops silently once the budget is
// exhausted, mirroring the depth-cap behavior.
func (rm recordMap) spendRenderBudget() bool {
	if rm.budget == nil {
		return true
	}
	if rm.budget.remaining <= 0 {
		return false
	}
	rm.budget.remaining--
	return true
}

// blockRenderBudget sizes the render budget for a record map with blockCount
// blocks. Legitimate pages render each block about once (transclusions and
// aliases re-render a handful), so 8x plus slack is generous while keeping the
// hostile exponential case bounded to ~1k renders.
func blockRenderBudget(blockCount int) int {
	return 8*blockCount + 1024
}

type block struct {
	ID             string         `json:"id"`
	Type           string         `json:"type"`
	CollectionID   string         `json:"collection_id"`
	ViewIDs        []string       `json:"view_ids"`
	ParentID       string         `json:"parent_id"`
	Properties     map[string]any `json:"properties"`
	Content        []string       `json:"content"`
	Format         map[string]any `json:"format"`
	CreatedTime    any            `json:"created_time"`
	LastEditedTime any            `json:"last_edited_time"`
}

type collection struct {
	ID     string                        `json:"id"`
	Name   any                           `json:"name"`
	Schema map[string]collectionProperty `json:"schema"`
}

type collectionProperty struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	NumberFormat string `json:"number_format"`
	ResultType   string `json:"result_type"`
	Prefix       string `json:"prefix"` // unique_id prefix ("TASK" renders as TASK-12)
	Formula      map[string]any
	OptionColors map[string]string
}

func (p *collectionProperty) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.Name = stringValue(raw["name"])
	p.Type = stringValue(raw["type"])
	p.NumberFormat = firstNonEmpty(stringValue(raw["number_format"]), stringValue(raw["format"]))
	p.ResultType = firstNonEmpty(stringValue(raw["result_type"]), stringValue(raw["value_type"]))
	p.Prefix = stringValue(raw["prefix"])
	if child, ok := raw["unique_id"].(map[string]any); ok {
		p.Prefix = firstNonEmpty(p.Prefix, stringValue(child["prefix"]))
	}
	p.OptionColors = collectionOptionColors(raw["options"])
	if child, ok := raw["formula"].(map[string]any); ok {
		p.Formula = child
		p.ResultType = firstNonEmpty(p.ResultType, stringValue(child["result_type"]), stringValue(child["value_type"]), stringValue(child["type"]))
		p.NumberFormat = firstNonEmpty(p.NumberFormat, stringValue(child["number_format"]), stringValue(child["format"]))
	}
	if child, ok := raw["rollup"].(map[string]any); ok {
		p.ResultType = firstNonEmpty(p.ResultType, stringValue(child["type"]), stringValue(child["result_type"]), stringValue(child["value_type"]))
		p.NumberFormat = firstNonEmpty(p.NumberFormat, stringValue(child["number_format"]), stringValue(child["format"]))
	}
	return nil
}

func collectionOptionColors(value any) map[string]string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	colors := map[string]string{}
	for _, item := range items {
		option, ok := item.(map[string]any)
		if !ok {
			continue
		}
		className := collectionOptionColorClass(stringValue(option["color"]))
		if className == "" {
			continue
		}
		for _, key := range []string{"value", "name", "id"} {
			label := strings.TrimSpace(stringValue(option[key]))
			if label == "" {
				continue
			}
			colors[label] = className
			colors[strings.ToLower(label)] = className
		}
	}
	if len(colors) == 0 {
		return nil
	}
	return colors
}

func collectionOptionColorClass(value string) string {
	color := notionColorClass(value)
	if color == "" {
		return ""
	}
	if strings.HasSuffix(color, "-background") {
		return "notion-color--" + color
	}
	return "notion-color--" + color + "-background"
}

type collectionView struct {
	ID     string         `json:"id"`
	Type   string         `json:"type"`
	Name   any            `json:"name"`
	Format map[string]any `json:"format"`
	Query  map[string]any `json:"query"`
	Query2 map[string]any `json:"query2"`
}

type collectionViewProperty struct {
	Property string
	Width    int
}

type collectionQuery struct {
	CollectionGroupResults collectionResults      `json:"collection_group_results"`
	BoardColumns           collectionBoardColumns `json:"board_columns"`
	Results                map[string]collectionResults
}

type collectionResults struct {
	BlockIDs           []string                      `json:"blockIds"`
	AggregationResults []collectionAggregationResult `json:"aggregationResults"`
}

type collectionAggregationResult struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type collectionBoardColumns struct {
	Results []collectionBoardColumn `json:"results"`
}

type collectionBoardColumn struct {
	Value   map[string]any `json:"value"`
	Hidden  bool           `json:"hidden"`
	Visible *bool          `json:"visible"`
}

type automation struct {
	ID         string               `json:"id"`
	ActionIDs  []string             `json:"action_ids"`
	Properties automationProperties `json:"properties"`
}

type automationProperties struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

type automationAction struct {
	ID     string                 `json:"id"`
	Type   string                 `json:"type"`
	Config automationActionConfig `json:"config"`
}

type automationActionConfig struct {
	Target automationActionTarget `json:"target"`
}

type automationActionTarget struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	PageID string `json:"pageId"`
}

func (q *collectionQuery) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	q.Results = map[string]collectionResults{}
	for key, value := range raw {
		switch key {
		case "collection_group_results":
			if err := json.Unmarshal(value, &q.CollectionGroupResults); err != nil {
				return err
			}
		case "board_columns":
			if err := json.Unmarshal(value, &q.BoardColumns); err != nil {
				return err
			}
		case "blockIds":
			var blockIDs []string
			if err := json.Unmarshal(value, &blockIDs); err != nil {
				return err
			}
			q.CollectionGroupResults.BlockIDs = blockIDs
		case "aggregationResults":
			var results []collectionAggregationResult
			if err := json.Unmarshal(value, &results); err != nil {
				return err
			}
			q.CollectionGroupResults.AggregationResults = results
		default:
			if strings.HasPrefix(key, "results:") {
				var results collectionResults
				if err := json.Unmarshal(value, &results); err != nil {
					return err
				}
				q.Results[key] = results
			}
		}
	}
	return nil
}

// SubpageLink is a child page reachable from a page, identified by its
// normalized PageID and its display Title.
type SubpageLink struct {
	PageID string
	Title  string
}

type subpageLinkCandidate struct {
	PageID string
	Title  string
}

type mentionResolver func(kind string, value any, rawText string) string

// RenderPage renders the page identified by input.PageID from input.RecordMap
// into a single self-contained, sanitized HTML string. Untrusted text is
// escaped and only URLs that pass the package's safety checks are emitted. It
// returns an error if the record map cannot be parsed or the root page is not
// present.
func RenderPage(input RenderInput) (string, error) {
	var rm recordMap
	if err := json.Unmarshal(input.RecordMap, &rm); err != nil {
		return "", err
	}
	rm.budget = &renderBudget{remaining: blockRenderBudget(len(rm.Block))}
	rootID := NormalizeID(input.PageID)
	root, ok := rm.Block[rootID]
	if !ok {
		return "", fmt.Errorf("notion root page %q not found", input.PageID)
	}

	var b strings.Builder
	b.WriteString(`<div class="`)
	b.WriteString(html.EscapeString(strings.Join(notionBodyClasses(root), " ")))
	b.WriteString(`">`)
	renderPageCover(&b, rm, root, input)
	renderPageIconHero(&b, rm, root, input)
	renderWarnings(&b, input.Warnings, input)
	renderBlockList(&b, rm, root.Content, input, 0)
	b.WriteString(`</div>`)
	return b.String(), nil
}

func renderWarnings(out *strings.Builder, warnings []RenderWarning, input RenderInput) {
	warnings = normalizeRenderWarnings(warnings)
	if len(warnings) == 0 {
		return
	}
	out.WriteString(`<div class="notion-render-warning" role="note"><span class="notion-render-warning__icon" aria-hidden="true">!</span><div class="notion-render-warning__body"><strong>`)
	out.WriteString(html.EscapeString(input.t("notion.render_warning_title", "Some Notion content could not be loaded.")))
	out.WriteString(`</strong><ul>`)
	for _, warning := range warnings {
		out.WriteString(`<li>`)
		out.WriteString(html.EscapeString(renderWarningText(warning, input)))
		out.WriteString(`</li>`)
	}
	out.WriteString(`</ul></div></div>`)
}

func normalizeRenderWarnings(warnings []RenderWarning) []RenderWarning {
	order := []string{
		RenderWarningFetchErrors,
		RenderWarningAssetDownloadErrors,
		RenderWarningAssetsTruncated,
		RenderWarningBlocksTruncated,
	}
	totals := map[string]int{}
	for _, warning := range warnings {
		switch warning.Kind {
		case RenderWarningFetchErrors, RenderWarningAssetDownloadErrors, RenderWarningAssetsTruncated, RenderWarningBlocksTruncated:
			count := warning.Count
			if count <= 0 {
				count = 1
			}
			totals[warning.Kind] += count
		}
	}
	out := make([]RenderWarning, 0, len(totals))
	for _, kind := range order {
		if count := totals[kind]; count > 0 {
			out = append(out, RenderWarning{Kind: kind, Count: count})
		}
	}
	return out
}

func renderWarningText(warning RenderWarning, loc RenderInput) string {
	switch warning.Kind {
	case RenderWarningFetchErrors:
		return fmt.Sprintf("%d %s", max(warning.Count, 1), loc.t("notion.warn_fetch", "fetch warnings"))
	case RenderWarningAssetDownloadErrors:
		return fmt.Sprintf("%d %s", max(warning.Count, 1), loc.t("notion.warn_asset_download", "asset download warnings"))
	case RenderWarningAssetsTruncated:
		return loc.t("notion.warn_assets_truncated", "Asset limit reached; some media may be unavailable.")
	case RenderWarningBlocksTruncated:
		return loc.t("notion.warn_blocks_truncated", "Block limit reached; some page content may be missing.")
	default:
		return "Notion render warning"
	}
}

func notionBodyClasses(root block) []string {
	classes := []string{"notion-body"}
	if boolValue(root.Format["page_full_width"]) {
		classes = append(classes, "notion-body--full-width")
	}
	if boolValue(root.Format["page_small_text"]) {
		classes = append(classes, "notion-body--small-text")
	}
	font := strings.ToLower(strings.TrimSpace(stringValue(root.Format["page_font"])))
	switch font {
	case "serif":
		classes = append(classes, "notion-body--font-serif")
	case "mono", "monospace", "code":
		classes = append(classes, "notion-body--font-mono")
	}
	return classes
}

func renderBlockList(out *strings.Builder, rm recordMap, ids []string, input RenderInput, depth int) {
	if depth > 32 {
		return
	}
	for i := 0; i < len(ids); {
		id := NormalizeID(ids[i])
		blk, ok := rm.Block[id]
		if !ok {
			i++
			continue
		}
		if isListBlock(blk.Type) {
			listType := blk.Type
			start := listStartIndex(blk)
			group := make([]string, 0, 4)
			for i < len(ids) {
				nextID := NormalizeID(ids[i])
				next, ok := rm.Block[nextID]
				if !ok || next.Type != listType {
					break
				}
				group = append(group, nextID)
				i++
			}
			renderList(out, rm, group, input, depth, listType, start)
			continue
		}
		renderBlock(out, rm, id, input, depth)
		i++
	}
}

func renderBlock(out *strings.Builder, rm recordMap, id string, input RenderInput, depth int) {
	if depth > 32 {
		return
	}
	if !rm.spendRenderBudget() {
		return
	}
	blk, ok := rm.Block[id]
	if !ok {
		return
	}
	text := richTextWithResolver(blk.Properties["title"], notionMentionResolver(rm, input))
	switch blk.Type {
	case "text":
		if renderTextBlock(out, rm, blk, input, depth, text) {
			return
		}
	case "header", "sub_header", "sub_sub_header", "header_4":
		if renderHeadingBlock(out, rm, blk, input, depth, text) {
			return
		}
	case "bulleted_list", "numbered_list":
		renderList(out, rm, []string{id}, input, depth, blk.Type, listStartIndex(blk))
	case "code":
		renderCode(out, rm, blk, input)
	case "equation":
		renderEquation(out, blk)
	case "table_of_contents":
		renderTableOfContents(out, rm, input)
	case "column_list":
		renderColumnListBlock(out, rm, blk, input, depth)
	case "column":
		renderColumnBlock(out, rm, blk, input, depth)
	case "synced_block", "transclusion_container":
		renderSyncedBlock(out, rm, blk, input, depth)
	case "transclusion_reference":
		renderTransclusionReference(out, rm, blk, input, depth)
	case "button":
		renderButtonBlock(out, rm, blk, input, text)
	case "tab":
		renderTabBlock(out, rm, blk, input, depth)
	case "breadcrumb":
		renderBreadcrumbBlock(out, rm, input)
	case "to_do":
		if renderTodoBlock(out, rm, blk, input, depth, text) {
			return
		}
	case "quote":
		renderQuoteBlock(out, rm, blk, input, depth, text)
	case "callout":
		renderCalloutBlock(out, rm, blk, input, depth, text)
	case "toggle":
		renderToggleBlock(out, rm, blk, input, depth, text)
	case "divider":
		out.WriteString(`<hr>`)
	case "image":
		renderImageBlock(out, rm, blk, input)
	case "video":
		renderVideoBlock(out, rm, blk, input, text)
	case "audio":
		renderAudioBlock(out, rm, blk, input, text)
	case "file", "pdf":
		renderFileBlock(out, rm, blk, input, text)
	case "bookmark":
		renderExternalLinkBlock(out, rm, blk, text, input)
	case "tweet":
		renderTweetBlock(out, rm, blk, text, input)
	case "drive":
		if !renderDriveCard(out, rm, blk, text, input) {
			renderEmbeddedOrExternalLinkBlock(out, rm, blk, text, input)
		}
	case "external_object_instance":
		if !renderExternalObjectBlock(out, rm, blk, text, input) {
			renderEmbeddedOrExternalLinkBlock(out, rm, blk, text, input)
		}
	case "embed", "gist", "figma", "typeform", "replit", "codepen", "excalidraw", "maps", "miro":
		renderEmbeddedOrExternalLinkBlock(out, rm, blk, text, input)
	case "page":
		renderLinkedPageBlock(out, rm, blk, input, text)
	case "table":
		renderTable(out, rm, blk, input)
	case "collection_view", "collection_view_page":
		renderCollectionView(out, rm, blk, text, input)
	case "alias":
		renderLinkedPageBlock(out, rm, blk, input, text)
	default:
		renderDefaultBlock(out, rm, blk, input, text)
	}

	if !blockRendersOwnChildren(blk.Type) {
		renderChildren(out, rm, blk, input, depth)
	}
}

// renderTextBlock renders a "text" (paragraph) block. It reports whether it
// fully handled the block (including its children), in which case the caller
// must return without falling through to the shared child rendering.
func renderTextBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) bool {
	if text == "" && len(blk.Content) == 0 {
		out.WriteString(`<p class="notion-blank" aria-hidden="true">&nbsp;</p>`)
		return true
	}
	if len(blk.Content) > 0 {
		out.WriteString(`<div class="notion-block-with-children notion-block-with-children--text">`)
		out.WriteString(`<p`)
		out.WriteString(blockClassAttr(blk))
		out.WriteString(`>`)
		out.WriteString(text)
		out.WriteString(`</p>`)
		renderChildBlockGroup(out, rm, blk, input, depth)
		out.WriteString(`</div>`)
		return true
	}
	out.WriteString(`<p`)
	out.WriteString(blockClassAttr(blk))
	out.WriteString(`>`)
	out.WriteString(text)
	out.WriteString(`</p>`)
	return false
}

// renderHeadingBlock renders a heading block. It reports whether the block was
// rendered as a toggle heading, which handles its own children and means the
// caller must return without falling through to the shared child rendering.
func renderHeadingBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) bool {
	level := notionHeadingLevel(blk.Type)
	if renderToggleHeading(out, rm, blk, input, depth, level, text) {
		return true
	}
	renderHeading(out, level, text, blk, input)
	return false
}

func renderColumnListBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	out.WriteString(`<div class="notion-column-list">`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</div>`)
}

func renderColumnBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	out.WriteString(`<div class="notion-column"`)
	if style := columnRatioStyle(blk); style != "" {
		out.WriteString(style)
	}
	out.WriteString(`>`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</div>`)
}

func renderSyncedBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	out.WriteString(`<div class="notion-synced-block">`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</div>`)
}

// renderTodoBlock renders a "to_do" block. It reports whether it fully handled
// the block (including its children), in which case the caller must return
// without falling through to the shared child rendering.
func renderTodoBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) bool {
	if len(blk.Content) > 0 {
		out.WriteString(`<div class="notion-block-with-children notion-block-with-children--todo">`)
		renderTodoLine(out, blk, text)
		renderChildBlockGroup(out, rm, blk, input, depth)
		out.WriteString(`</div>`)
		return true
	}
	renderTodoLine(out, blk, text)
	return false
}

func renderQuoteBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) {
	out.WriteString(`<blockquote`)
	out.WriteString(blockClassAttr(blk))
	out.WriteString(`>`)
	if len(blk.Content) > 0 {
		out.WriteString(`<p>`)
		out.WriteString(text)
		out.WriteString(`</p>`)
		renderChildBlockGroup(out, rm, blk, input, depth)
	} else {
		out.WriteString(text)
	}
	out.WriteString(`</blockquote>`)
}

func renderCalloutBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) {
	out.WriteString(`<div class="notion-callout`)
	if color := notionBlockColorClass(blk); color != "" {
		out.WriteString(` `)
		out.WriteString(html.EscapeString(color))
	}
	out.WriteString(`">`)
	if icon, _ := blk.Format["page_icon"].(string); icon != "" {
		var iconHTML strings.Builder
		if renderIconValue(&iconHTML, icon, resolvedAssetURL(input, rm, blk.ID, "icon", icon)) {
			out.WriteString(`<span class="notion-callout__icon">`)
			out.WriteString(iconHTML.String())
			out.WriteString(`</span>`)
		}
	}
	out.WriteString(`<div class="notion-callout__content">`)
	out.WriteString(text)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</div></div>`)
}

func renderToggleBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, text string) {
	out.WriteString(`<details`)
	out.WriteString(blockClassAttr(blk, "notion-toggle"))
	out.WriteString(`><summary>`)
	out.WriteString(firstText(text, input.t("notion.toggle_details", "Details")))
	out.WriteString(`</summary>`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</details>`)
}

func renderImageBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput) {
	source := blockSource(blk)
	src := resolvedAssetSourceURL(input, rm, blk.ID, source)
	if src == "" {
		src = safeExternalImageURL(source)
	}
	if src != "" {
		renderMediaFigureStart(out, blk, "image")
		alt := plainText(blk.Properties["caption"])
		if strings.TrimSpace(alt) == "" {
			alt = fileLabel(source)
		}
		renderLightboxImage(out, "notion-media__link", src, alt)
		if caption := richTextWithResolver(blk.Properties["caption"], notionMentionResolver(rm, input)); caption != "" {
			out.WriteString(`<figcaption>`)
			out.WriteString(caption)
			out.WriteString(`</figcaption>`)
		}
		out.WriteString(`</figure>`)
	}
}

func renderVideoBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	source := blockSource(blk)
	if src := resolvedAssetSourceURL(input, rm, blk.ID, source); src != "" {
		renderMediaFigureStart(out, blk, "video")
		out.WriteString(`<video controls preload="metadata"`)
		out.WriteString(attr("src", src))
		// Always give the element an accessible name (the caption text, or the
		// file label) — figcaption alone does not name the video, mirroring
		// how images always carry alt text. attr escapes the plain-text value.
		caption := strings.TrimSpace(plainText(blk.Properties["caption"]))
		out.WriteString(attr("aria-label", firstNonEmpty(caption, fileLabel(source))))
		out.WriteString(`></video>`)
		renderCaptionOrLabel(out, rm, blk, input, firstText(text, fileLabel(source)))
		out.WriteString(`</figure>`)
	} else if isNotionAssetURL(source) {
		renderFilePlaceholder(out, firstText(text, fileLabel(source)))
	} else {
		renderEmbeddedOrExternalLinkBlock(out, rm, blk, text, input)
	}
}

func renderAudioBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	source := blockSource(blk)
	if src := resolvedAssetSourceURL(input, rm, blk.ID, source); src != "" {
		renderMediaFigureStart(out, blk, "audio")
		out.WriteString(`<audio controls preload="metadata"`)
		out.WriteString(attr("src", src))
		caption := strings.TrimSpace(plainText(blk.Properties["caption"]))
		out.WriteString(attr("aria-label", firstNonEmpty(caption, fileLabel(source))))
		out.WriteString(`></audio>`)
		renderCaptionOrLabel(out, rm, blk, input, firstText(text, fileLabel(source)))
		out.WriteString(`</figure>`)
	} else if isNotionAssetURL(source) {
		renderFilePlaceholder(out, firstText(text, fileLabel(source)))
	} else {
		renderExternalLinkBlock(out, rm, blk, text, input)
	}
}

func renderFileBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	source := blockSource(blk)
	if src := resolvedAssetSourceURL(input, rm, blk.ID, source); src != "" {
		if isLocalPDFURL(src) {
			renderPDFEmbed(out, src, firstText(text, pdfFilename(src)), input)
		} else {
			renderFileLink(out, src, firstText(text, input.t("notion.download_file", "Download file")))
		}
	} else if isNotionAssetURL(source) {
		renderFilePlaceholder(out, firstText(text, fileLabel(source)))
	} else if src := safeURL(source); src != "" {
		renderFileLink(out, src, firstText(text, input.t("notion.download_file", "Download file")))
	}
}

func renderDefaultBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	if blockSource(blk) != "" {
		renderExternalLinkBlock(out, rm, blk, text, input)
	} else if text != "" {
		out.WriteString(`<p>`)
		out.WriteString(text)
		out.WriteString(`</p>`)
	}
}

func renderTodoLine(out *strings.Builder, blk block, text string) {
	checked := todoChecked(blk)
	out.WriteString(`<p`)
	if checked {
		out.WriteString(blockClassAttr(blk, "notion-todo", "notion-todo--checked"))
	} else {
		out.WriteString(blockClassAttr(blk, "notion-todo"))
	}
	out.WriteString(`>`)
	out.WriteString(`<input type="checkbox" disabled`)
	if checked {
		out.WriteString(` checked`)
	}
	out.WriteString(`> `)
	out.WriteString(`<span class="notion-todo__text">`)
	out.WriteString(text)
	out.WriteString(`</span>`)
	out.WriteString(`</p>`)
}

func renderChildren(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	renderBlockList(out, rm, blk.Content, input, depth+1)
}

func renderChildBlockGroup(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	if len(blk.Content) == 0 {
		return
	}
	out.WriteString(`<div class="notion-block-children">`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</div>`)
}

func isListBlock(blockType string) bool {
	return blockType == "bulleted_list" || blockType == "numbered_list"
}

func renderList(out *strings.Builder, rm recordMap, ids []string, input RenderInput, depth int, listType string, start int) {
	if len(ids) == 0 {
		return
	}
	tag := "ul"
	if listType == "numbered_list" {
		tag = "ol"
	}
	out.WriteString(`<`)
	out.WriteString(tag)
	if tag == "ol" && start > 1 {
		out.WriteString(` start="`)
		out.WriteString(fmt.Sprint(start))
		out.WriteString(`"`)
	}
	out.WriteString(`>`)
	for _, id := range ids {
		blk, ok := rm.Block[id]
		if !ok {
			continue
		}
		if !rm.spendRenderBudget() {
			break
		}
		out.WriteString(`<li`)
		out.WriteString(blockClassAttr(blk))
		out.WriteString(`>`)
		out.WriteString(richTextWithResolver(blk.Properties["title"], notionMentionResolver(rm, input)))
		renderChildren(out, rm, blk, input, depth)
		out.WriteString(`</li>`)
	}
	out.WriteString(`</`)
	out.WriteString(tag)
	out.WriteString(`>`)
}

func listStartIndex(blk block) int {
	return max(1, intValue(blk.Format["list_start_index"]))
}

func blockRendersOwnChildren(blockType string) bool {
	switch blockType {
	case "text", "to_do", "quote", "callout", "toggle", "table", "page", "alias", "bulleted_list", "numbered_list", "column_list", "column", "synced_block", "transclusion_container", "transclusion_reference", "table_of_contents", "collection_view", "collection_view_page", "tab", "breadcrumb":
		return true
	default:
		return false
	}
}
