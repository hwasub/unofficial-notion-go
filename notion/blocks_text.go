package notion

import (
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

type notionBreadcrumbItem struct {
	PageID string
	Title  string
	Href   string
}

func renderBreadcrumbBlock(out *strings.Builder, rm recordMap, input RenderInput) {
	items := notionBreadcrumbItems(rm, input)
	if len(items) == 0 {
		return
	}
	out.WriteString(`<nav class="notion-breadcrumb" aria-label="`)
	out.WriteString(html.EscapeString(input.t("nav.breadcrumb", "Breadcrumb")))
	out.WriteString(`"><ol>`)
	for i, item := range items {
		current := i == len(items)-1
		if current {
			out.WriteString(`<li aria-current="page"><span>`)
			out.WriteString(html.EscapeString(item.Title))
			out.WriteString(`</span></li>`)
			continue
		}
		out.WriteString(`<li>`)
		if item.Href != "" {
			out.WriteString(`<a`)
			out.WriteString(attr("href", item.Href))
			out.WriteString(`>`)
			out.WriteString(html.EscapeString(item.Title))
			out.WriteString(`</a>`)
		} else {
			out.WriteString(`<span>`)
			out.WriteString(html.EscapeString(item.Title))
			out.WriteString(`</span>`)
		}
		out.WriteString(`</li>`)
	}
	out.WriteString(`</ol></nav>`)
}

func notionBreadcrumbItems(rm recordMap, input RenderInput) []notionBreadcrumbItem {
	currentID := NormalizeID(input.PageID)
	if currentID == "" {
		return nil
	}
	seen := map[string]struct{}{}
	items := make([]notionBreadcrumbItem, 0, 4)
	for id := currentID; id != "" && len(items) < 12; {
		if _, ok := seen[id]; ok {
			break
		}
		seen[id] = struct{}{}
		page, ok := rm.Block[id]
		if !ok || page.Type != "page" {
			break
		}
		items = append(items, notionBreadcrumbItem{
			PageID: id,
			Title:  firstNonEmpty(plainText(page.Properties["title"]), "Page"),
			Href:   notionPageHrefForInput(input, id),
		})
		id = NormalizeID(page.ParentID)
	}
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
	return items
}

func renderLinkedPageBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	pageID, ok := linkedPageID(blk)
	title := firstNonEmpty(text, linkedPageTitle(rm, pageID), escapeText(input.t("notion.untitled", "Untitled")))
	if ok {
		if href := notionPageHrefForInput(input, pageID); href != "" {
			out.WriteString(`<p class="notion-subpage"><a`)
			out.WriteString(attr("href", href))
			out.WriteString(`>`)
			renderLinkedPageIcon(out, rm, blk, pageID, input)
			out.WriteString(title)
			out.WriteString(`</a></p>`)
			return
		}
	}
	if title != "" {
		out.WriteString(`<p class="notion-alias">`)
		renderLinkedPageIcon(out, rm, blk, pageID, input)
		out.WriteString(title)
		out.WriteString(`</p>`)
	}
}

func renderLinkedPageIcon(out *strings.Builder, rm recordMap, blk block, pageID string, input RenderInput) {
	if page, ok := rm.Block[pageID]; ok && page.Type == "page" {
		renderPageIcon(out, rm, page, input)
		return
	}
	renderPageIcon(out, rm, blk, input)
}

func linkedPageTitle(rm recordMap, pageID string) string {
	page, ok := rm.Block[pageID]
	if !ok || page.Type != "page" {
		return ""
	}
	return richText(page.Properties["title"])
}

func linkedPageID(blk block) (string, bool) {
	if blk.Type == "page" {
		return normalizedPageID(blk.ID)
	}
	for _, key := range []string{"alias_pointer", "page_pointer", "link_to_page", "block_pointer"} {
		if pageID, ok := normalizedPageID(pointerID(blk.Format[key])); ok {
			return pageID, true
		}
	}
	for _, key := range []string{"page_id", "target_page_id"} {
		if pageID, ok := normalizedPageID(stringValue(blk.Format[key])); ok {
			return pageID, true
		}
	}
	return "", false
}

func renderHeading(out *strings.Builder, level int, text string, blk block, input RenderInput) {
	if level < 1 || level > 4 {
		level = 3
	}
	tag := "h" + fmt.Sprint(level)
	out.WriteString(`<`)
	out.WriteString(tag)
	out.WriteString(blockClassAttr(blk))
	if anchor := headingAnchor(blk.ID); anchor != "" {
		out.WriteString(` id="`)
		out.WriteString(html.EscapeString(anchor))
		out.WriteString(`"`)
	}
	out.WriteString(`>`)
	out.WriteString(firstText(text, input.t("notion.untitled", "Untitled")))
	out.WriteString(`</`)
	out.WriteString(tag)
	out.WriteString(`>`)
}

func renderToggleHeading(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int, level int, text string) bool {
	if !boolValue(blk.Format["toggleable"]) {
		return false
	}
	if level < 1 || level > 4 {
		level = 3
	}
	out.WriteString(`<details`)
	out.WriteString(blockClassAttr(blk, "notion-toggle-heading", "notion-toggle-heading--h"+fmt.Sprint(level)))
	if anchor := headingAnchor(blk.ID); anchor != "" {
		out.WriteString(` id="`)
		out.WriteString(html.EscapeString(anchor))
		out.WriteString(`"`)
	}
	headingTag := "h" + fmt.Sprint(level)
	out.WriteString(`><summary><`)
	out.WriteString(headingTag)
	out.WriteString(` class="notion-toggle-heading__title">`)
	out.WriteString(firstText(text, input.t("notion.toggle_details", "Details")))
	out.WriteString(`</`)
	out.WriteString(headingTag)
	out.WriteString(`></summary>`)
	renderChildren(out, rm, blk, input, depth)
	out.WriteString(`</details>`)
	return true
}

func notionHeadingLevel(blockType string) int {
	// Page titles already occupy h1 in the public template. Notion content
	// headings therefore start at h2.
	switch blockType {
	case "header":
		return 2
	case "sub_header":
		return 3
	case "sub_sub_header":
		return 4
	case "header_4":
		return 4
	default:
		return 3
	}
}

func blockClassAttr(blk block, classes ...string) string {
	if color := notionBlockColorClass(blk); color != "" {
		classes = append(classes, color)
	}
	if len(classes) == 0 {
		return ""
	}
	return ` class="` + html.EscapeString(strings.Join(classes, " ")) + `"`
}

func renderButtonBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, text string) {
	classes := []string{"notion-button"}
	if color := notionBlockColorClass(blk); color != "" {
		classes = append(classes, color)
	}
	label := buttonLabelHTML(rm, blk, text)
	if href := buttonActionHref(rm, blk, input); href != "" {
		out.WriteString(`<div class="notion-button-block"><a class="`)
		out.WriteString(html.EscapeString(strings.Join(append(classes, "notion-button--link"), " ")))
		out.WriteString(`"`)
		out.WriteString(attr("href", href))
		if !strings.HasPrefix(href, "/") {
			out.WriteString(` rel="noopener noreferrer"`)
		}
		out.WriteString(`><span class="notion-button__label">`)
		out.WriteString(label)
		out.WriteString(`</span></a></div>`)
		return
	}
	out.WriteString(`<div class="notion-button-block"><button type="button" class="`)
	out.WriteString(html.EscapeString(strings.Join(append(classes, "notion-button--inert"), " ")))
	out.WriteString(`" disabled aria-disabled="true"><span class="notion-button__label">`)
	out.WriteString(label)
	out.WriteString(`</span></button></div>`)
}

func buttonLabelHTML(rm recordMap, blk block, text string) string {
	if strings.TrimSpace(text) != "" {
		return text
	}
	if automation, ok := automationForBlock(rm, blk); ok && strings.TrimSpace(automation.Properties.Name) != "" {
		return html.EscapeString(automation.Properties.Name)
	}
	return html.EscapeString("Button")
}

func buttonActionHref(rm recordMap, blk block, input RenderInput) string {
	automation, ok := automationForBlock(rm, blk)
	if !ok || len(automation.ActionIDs) == 0 {
		return ""
	}
	action, ok := automationActionByID(rm, automation.ActionIDs[0])
	if !ok || action.Type != "open_page" {
		return ""
	}
	switch action.Config.Target.Type {
	case "url":
		return safeButtonURL(action.Config.Target.URL)
	case "page":
		pageID, ok := normalizedPageID(action.Config.Target.PageID)
		if !ok {
			return ""
		}
		return notionPageHrefForInput(input, pageID)
	default:
		return ""
	}
}

func automationForBlock(rm recordMap, blk block) (automation, bool) {
	return automationByID(rm, stringValue(blk.Format["automation_id"]))
}

func automationByID(rm recordMap, id string) (automation, bool) {
	if len(rm.Automation) == 0 {
		return automation{}, false
	}
	for _, candidate := range idCandidates(id) {
		if automation, ok := rm.Automation[candidate]; ok {
			return automation, true
		}
	}
	return automation{}, false
}

func automationActionByID(rm recordMap, id string) (automationAction, bool) {
	if len(rm.AutomationAction) == 0 {
		return automationAction{}, false
	}
	for _, candidate := range idCandidates(id) {
		if action, ok := rm.AutomationAction[candidate]; ok {
			return action, true
		}
	}
	return automationAction{}, false
}

func idCandidates(id string) []string {
	normalized := NormalizeID(id)
	compact := strings.ReplaceAll(normalized, "-", "")
	out := []string{}
	for _, candidate := range []string{id, normalized, compact} {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		duplicate := false
		for _, existing := range out {
			if existing == candidate {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, candidate)
		}
	}
	return out
}

func columnRatioStyle(blk block) string {
	ratio, ok := numberValue(blk.Format["column_ratio"])
	if !ok || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0 {
		return ""
	}
	ratio = min(max(ratio, 0.05), 1)
	return ` style="--notion-column-ratio:` + strconv.FormatFloat(ratio, 'f', 4, 64) + `"`
}

func renderTabBlock(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	tabIDs := make([]string, 0, len(blk.Content))
	for _, rawID := range blk.Content {
		id := NormalizeID(rawID)
		if _, ok := rm.Block[id]; ok {
			tabIDs = append(tabIDs, id)
		}
	}
	if len(tabIDs) == 0 {
		out.WriteString(`<div class="notion-tab-block notion-blank" aria-hidden="true">&nbsp;</div>`)
		return
	}
	base := strings.ReplaceAll(NormalizeID(blk.ID), "-", "")
	if base == "" {
		base = "block"
	}
	out.WriteString(`<div class="notion-tab-block" data-notion-tabs>`)
	out.WriteString(`<div class="notion-tab-list" role="tablist" aria-label="`)
	out.WriteString(html.EscapeString(input.t("notion.tabs", "Tabs")))
	out.WriteString(`">`)
	for i, tabID := range tabIDs {
		tab := rm.Block[tabID]
		label := plainText(tab.Properties["title"])
		if label == "" {
			label = "Tab " + strconv.Itoa(i+1)
		}
		buttonID, panelID := notionTabIDs(base, i)
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
		out.WriteString(`"><span class="notion-tab-button__label">`)
		out.WriteString(html.EscapeString(label))
		out.WriteString(`</span></button>`)
	}
	out.WriteString(`</div>`)
	for i, tabID := range tabIDs {
		tab := rm.Block[tabID]
		buttonID, panelID := notionTabIDs(base, i)
		out.WriteString(`<section class="notion-tab-panel" role="tabpanel" tabindex="0" id="`)
		out.WriteString(html.EscapeString(panelID))
		out.WriteString(`" aria-labelledby="`)
		out.WriteString(html.EscapeString(buttonID))
		out.WriteString(`" data-notion-tab-panel`)
		if i > 0 {
			out.WriteString(` hidden`)
		}
		out.WriteString(`>`)
		if len(tab.Content) > 0 {
			renderBlockList(out, rm, tab.Content, input, depth+1)
		} else {
			out.WriteString(`<p class="notion-blank" aria-hidden="true">&nbsp;</p>`)
		}
		out.WriteString(`</section>`)
	}
	out.WriteString(`</div>`)
}

func notionTabIDs(base string, index int) (string, string) {
	suffix := strconv.Itoa(index)
	return "notion-tab-" + base + "-tab-" + suffix, "notion-tab-" + base + "-panel-" + suffix
}

func renderTransclusionReference(out *strings.Builder, rm recordMap, blk block, input RenderInput, depth int) {
	pointer := pointerID(blk.Format["transclusion_reference_pointer"])
	container, ok := rm.Block[pointer]
	if !ok || len(container.Content) == 0 {
		out.WriteString(`<div class="notion-synced-block"></div>`)
		return
	}
	out.WriteString(`<div class="notion-synced-block">`)
	renderBlockList(out, rm, container.Content, input, depth+1)
	out.WriteString(`</div>`)
}

func headingAnchor(id string) string {
	normalized, ok := normalizedPageID(id)
	if !ok {
		normalized = NormalizeID(id)
	}
	compact := strings.ReplaceAll(strings.TrimSpace(normalized), "-", "")
	if compact == "" {
		return ""
	}
	return "notion-" + compact
}

func renderCode(out *strings.Builder, rm recordMap, blk block, input RenderInput) {
	code := plainText(blk.Properties["title"])
	languageLabel := plainText(blk.Properties["language"])
	if languageLabel == "" {
		languageLabel, _ = blk.Format["code_language"].(string)
	}
	language := sanitizeClassToken(languageLabel)
	languageDisplay := codeLanguageDisplayLabel(languageLabel)
	out.WriteString(`<div class="notion-code">`)
	out.WriteString(`<div class="notion-code__bar">`)
	if languageDisplay != "" {
		out.WriteString(`<span class="notion-code__language">`)
		out.WriteString(html.EscapeString(languageDisplay))
		out.WriteString(`</span>`)
	}
	out.WriteString(`<button type="button" class="notion-code__copy" data-copy-code data-copy-success="`)
	out.WriteString(html.EscapeString(input.t("notion.copied", "Copied")))
	out.WriteString(`" data-copy-prompt="`)
	out.WriteString(html.EscapeString(input.t("notion.copy_code", "Copy code")))
	out.WriteString(`">`)
	out.WriteString(html.EscapeString(input.t("notion.copy", "Copy")))
	out.WriteString(`</button>`)
	out.WriteString(`</div>`)
	// Code is emitted as a plain, escaped <pre><code> with a language-* class.
	// Syntax coloring is left to the caller's stylesheet or a client-side
	// highlighter (e.g. Prism.js), which keeps this library dependency-free.
	out.WriteString(`<pre><code`)
	if language != "" {
		out.WriteString(` class="language-`)
		out.WriteString(html.EscapeString(language))
		out.WriteString(`"`)
	}
	out.WriteString(`>`)
	out.WriteString(html.EscapeString(code))
	out.WriteString(`</code></pre>`)
	if caption := richTextWithResolver(blk.Properties["caption"], notionMentionResolver(rm, input)); caption != "" {
		out.WriteString(`<div class="notion-code__caption">`)
		out.WriteString(caption)
		out.WriteString(`</div>`)
	}
	out.WriteString(`</div>`)
}

func codeLanguageDisplayLabel(label string) string {
	label = strings.Join(strings.Fields(strings.TrimSpace(label)), " ")
	runes := []rune(label)
	if len(runes) > 64 {
		label = string(runes[:64])
	}
	return label
}

func renderEquation(out *strings.Builder, blk block) {
	equation := equationText(firstNonNil(blk.Properties["equation"], blk.Properties["title"]))
	if equation == "" {
		return
	}
	escaped := html.EscapeString(equation)
	out.WriteString(`<div class="notion-equation" role="math" data-tex="`)
	out.WriteString(escaped)
	out.WriteString(`">`)
	out.WriteString(escaped)
	out.WriteString(`</div>`)
}

func renderTableOfContents(out *strings.Builder, rm recordMap, input RenderInput) {
	root, ok := rm.Block[NormalizeID(input.PageID)]
	if !ok {
		return
	}
	type item struct {
		level int
		id    string
		title string
	}
	items := make([]item, 0)
	seen := map[string]struct{}{}
	var walk func([]string)
	walk = func(ids []string) {
		for _, rawID := range ids {
			id := NormalizeID(rawID)
			// Guard against block-content cycles, which would otherwise drive
			// walk into unbounded recursion (the page renderer has a depth cap,
			// but this traversal does not).
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			blk, ok := rm.Block[id]
			if !ok {
				continue
			}
			switch blk.Type {
			case "header", "sub_header", "sub_sub_header", "header_4":
				title := plainText(blk.Properties["title"])
				if title != "" {
					items = append(items, item{level: headingLevel(blk.Type), id: headingAnchor(blk.ID), title: title})
				}
			case "page":
				continue
			}
			walk(blk.Content)
		}
	}
	walk(root.Content)
	if len(items) == 0 {
		return
	}
	out.WriteString(`<nav class="notion-toc"><ol>`)
	for _, item := range items {
		out.WriteString(`<li class="notion-toc__item notion-toc__item--level-`)
		out.WriteString(fmt.Sprint(item.level))
		out.WriteString(`"><a`)
		out.WriteString(attr("href", "#"+item.id))
		out.WriteString(`>`)
		out.WriteString(html.EscapeString(item.title))
		out.WriteString(`</a></li>`)
	}
	out.WriteString(`</ol></nav>`)
}

func headingLevel(blockType string) int {
	return notionHeadingLevel(blockType)
}

func renderCaptionOrLabel(out *strings.Builder, rm recordMap, blk block, input RenderInput, labelHTML string) {
	if caption := richTextWithResolver(blk.Properties["caption"], notionMentionResolver(rm, input)); caption != "" {
		out.WriteString(`<figcaption>`)
		out.WriteString(caption)
		out.WriteString(`</figcaption>`)
		return
	}
	if strings.TrimSpace(labelHTML) == "" {
		return
	}
	out.WriteString(`<figcaption>`)
	out.WriteString(labelHTML)
	out.WriteString(`</figcaption>`)
}
