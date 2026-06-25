package notion

import (
	"fmt"
	"html"
	"strings"
)

func renderPageCover(out *strings.Builder, rm recordMap, root block, input RenderInput) {
	cover := stringValue(root.Format["page_cover"])
	src := resolvedAssetURL(input, rm, root.ID, "cover", cover)
	if src == "" {
		src = safeExternalImageURL(cover)
	}
	if src != "" {
		out.WriteString(`<figure class="notion-cover notion-media notion-media--full"`)
		if style := coverStyle(root); style != "" {
			out.WriteString(` style="`)
			out.WriteString(style)
			out.WriteString(`"`)
		}
		out.WriteString(`><img`)
		out.WriteString(attr("src", src))
		out.WriteString(` loading="lazy" decoding="async" alt=""></figure>`)
	}
}

func renderMediaFigureStart(out *strings.Builder, blk block, kind string) {
	classes := []string{"notion-media", "notion-media--" + sanitizeClassToken(kind)}
	if alignment := mediaAlignment(blk); alignment != "" {
		classes = append(classes, "notion-media--"+alignment)
	}
	if boolValue(blk.Format["block_full_width"]) {
		classes = append(classes, "notion-media--full")
	}
	out.WriteString(`<figure class="`)
	out.WriteString(html.EscapeString(strings.Join(classes, " ")))
	out.WriteString(`"`)
	if style := mediaStyle(blk); style != "" {
		out.WriteString(` style="`)
		out.WriteString(style)
		out.WriteString(`"`)
	}
	out.WriteString(`>`)
}

func mediaAlignment(blk block) string {
	alignment := strings.ToLower(strings.TrimSpace(stringValue(blk.Format["block_alignment"])))
	switch alignment {
	case "left", "center", "right":
		return alignment
	default:
		return ""
	}
}

func mediaStyle(blk block) string {
	styles := make([]string, 0, 2)
	if width, ok := numberValue(blk.Format["block_width"]); ok && width > 0 && width <= 2000 {
		styles = append(styles, fmt.Sprintf("--notion-media-width:%.0fpx", width))
	}
	if ratio, ok := numberValue(blk.Format["block_aspect_ratio"]); ok && ratio > 0.05 && ratio <= 10 {
		// Notion stores block_aspect_ratio as height/width, but the CSS
		// aspect-ratio property consuming this variable is width/height, so
		// emit the reciprocal. The guard above bounds the raw h/w value.
		styles = append(styles, fmt.Sprintf("--notion-media-aspect-ratio:%.4g", 1/ratio))
	}
	return strings.Join(styles, ";")
}

func coverStyle(root block) string {
	position, ok := numberValue(root.Format["page_cover_position"])
	if !ok || position < 0 || position > 1 {
		return ""
	}
	// Notion's stored position maps to an inverted cover offset for CSS
	// object-position. Keep this here so callers do not need to understand the
	// page format quirk.
	return fmt.Sprintf("--notion-cover-position-y:%.2f%%", (1-position)*100)
}

func renderPageIconHero(out *strings.Builder, rm recordMap, root block, input RenderInput) {
	icon, _ := root.Format["page_icon"].(string)
	if icon == "" {
		return
	}
	var iconHTML strings.Builder
	if !renderIconValue(&iconHTML, icon, resolvedAssetURL(input, rm, root.ID, "icon", icon)) {
		return
	}
	out.WriteString(`<div class="notion-page-icon-hero" aria-hidden="true">`)
	out.WriteString(iconHTML.String())
	out.WriteString(`</div>`)
}

// renderPageTitle emits the page's own <h1> from the root block's title. It is
// the single h1 of the rendered body; Notion content headings start at h2 (see
// notionHeadingLevel). An untitled page emits nothing, preserving the bare body.
func renderPageTitle(out *strings.Builder, rm recordMap, root block, input RenderInput) {
	title := richTextWithResolver(root.Properties["title"], notionMentionResolver(rm, input))
	if strings.TrimSpace(title) == "" {
		return
	}
	out.WriteString(`<h1 class="notion-page-title">`)
	out.WriteString(title)
	out.WriteString(`</h1>`)
}

func renderPageIcon(out *strings.Builder, rm recordMap, blk block, input RenderInput) {
	icon, _ := blk.Format["page_icon"].(string)
	if icon == "" {
		return
	}
	var iconHTML strings.Builder
	if !renderIconValue(&iconHTML, icon, resolvedAssetURL(input, rm, blk.ID, "icon", icon)) {
		return
	}
	out.WriteString(`<span class="notion-page-icon" aria-hidden="true">`)
	out.WriteString(iconHTML.String())
	out.WriteString(`</span>`)
}

func renderIconValue(out *strings.Builder, icon string, localAsset string) bool {
	if src := safeURL(localAsset); src != "" {
		out.WriteString(`<img`)
		out.WriteString(attr("src", src))
		out.WriteString(` alt="">`)
		return true
	}
	if src := safeURL(icon); src != "" && !isNotionHostedURL(src) {
		out.WriteString(`<img`)
		out.WriteString(attr("src", src))
		out.WriteString(` alt="">`)
		return true
	}
	if safeURL(icon) != "" {
		return false
	}
	out.WriteString(html.EscapeString(icon))
	return strings.TrimSpace(icon) != ""
}

func pageIconHTML(rm recordMap, blk block, input RenderInput) string {
	var b strings.Builder
	renderPageIcon(&b, rm, blk, input)
	return b.String()
}
