package notion

import (
	"bytes"
	"fmt"
	"html"
	"net/url"
	"strings"
)

func richText(value any) string {
	return richTextWithResolver(value, nil)
}

func richTextWithResolver(value any, resolver mentionResolver) string {
	if value == nil {
		return ""
	}
	raw, ok := value.([]any)
	if !ok {
		return html.EscapeString(strings.TrimSpace(fmt.Sprint(value)))
	}
	var b bytes.Buffer
	for _, part := range raw {
		arr, ok := part.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		text, _ := arr[0].(string)
		escaped := escapeRichTextText(text)
		if len(arr) > 1 {
			escaped = applyDecorations(escaped, text, arr[1], resolver)
		}
		b.WriteString(escaped)
	}
	return strings.TrimSpace(b.String())
}

func plainText(value any) string {
	if value == nil {
		return ""
	}
	raw, ok := value.([]any)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	var b strings.Builder
	for _, part := range raw {
		arr, ok := part.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		if text, ok := arr[0].(string); ok {
			b.WriteString(text)
		}
	}
	return strings.TrimSpace(b.String())
}

func escapeRichTextText(text string) string {
	if text == "" {
		return ""
	}
	parts := strings.Split(text, "\n")
	for i := range parts {
		parts[i] = html.EscapeString(parts[i])
	}
	return strings.Join(parts, "<br>")
}

func applyDecorations(text string, rawText string, decorations any, resolver mentionResolver) string {
	raw, ok := decorations.([]any)
	if !ok {
		return text
	}
	bold := false
	italic := false
	strike := false
	code := false
	underline := false
	highlight := ""
	textColor := ""
	href := ""
	for _, deco := range raw {
		item, ok := deco.([]any)
		if !ok || len(item) == 0 {
			continue
		}
		kind, _ := item[0].(string)
		switch kind {
		case "b":
			bold = true
		case "i":
			italic = true
		case "s":
			strike = true
		case "c":
			code = true
		case "_":
			underline = true
		case "u":
			text = notionMentionHTML("Person", rawText)
		case "p":
			text = resolvedMentionHTML(resolver, "p", item, rawText, "Page")
		case "eoi":
			text = resolvedMentionHTML(resolver, "eoi", item, rawText, "External object")
		case "ce":
			if len(item) > 1 && resolver != nil {
				if rendered := resolver("ce", item[1], rawText); rendered != "" {
					text = rendered
				}
			}
		case "‣":
			if len(item) > 1 {
				if rendered := externalLinkMentionHTML(item[1], rawText); rendered != "" {
					text = rendered
				}
			}
		case "lm":
			if len(item) > 1 {
				if resolver != nil {
					if rendered := resolver("lm", item[1], rawText); rendered != "" {
						text = rendered
						continue
					}
				}
				if rendered := linkMentionHTMLValue(item[1], rawText, nil, nil); rendered != "" {
					text = rendered
				}
			}
		case "h":
			if len(item) > 1 {
				if color := notionColorClass(fmt.Sprint(item[1])); color != "" {
					// A "_background" color is a highlight (<mark>); a bare color
					// is a foreground text color (<span>).
					if strings.HasSuffix(color, "-background") {
						highlight = color
					} else {
						textColor = color
					}
				}
			}
		case "a":
			if len(item) > 1 {
				if value, ok := item[1].(string); ok {
					if resolver != nil {
						if internal := resolver("a", value, rawText); internal != "" {
							href = internal
							continue
						}
					}
					if safe := safeRichTextLinkURL(value); safe != "" {
						href = safe
					}
				}
			}
		case "d":
			if len(item) > 1 {
				if label, datetime := formatDateMention(item[1]); label != "" {
					text = `<span class="notion-date-mention"><time datetime="` + html.EscapeString(datetime) + `">` + html.EscapeString(label) + `</time></span>`
				}
			}
		case "e":
			equation := ""
			if len(item) > 1 {
				equation = strings.TrimSpace(fmt.Sprint(item[1]))
			}
			if equation == "" && strings.TrimSpace(rawText) != "" && strings.TrimSpace(rawText) != "‣" {
				equation = strings.TrimSpace(rawText)
			}
			equation = cleanEquationText(equation)
			if equation != "" {
				escapedEquation := html.EscapeString(equation)
				text = `<span class="notion-equation notion-equation--inline" role="math" data-tex="` + escapedEquation + `">` + escapedEquation + `</span>`
			}
		}
	}
	if code {
		text = "<code>" + text + "</code>"
	}
	if bold {
		text = "<strong>" + text + "</strong>"
	}
	if italic {
		text = "<em>" + text + "</em>"
	}
	if strike {
		text = "<s>" + text + "</s>"
	}
	if underline {
		text = `<span class="notion-underline">` + text + `</span>`
	}
	if textColor != "" {
		text = `<span class="notion-color--` + textColor + `">` + text + `</span>`
	}
	if highlight != "" {
		text = `<mark class="notion-highlight notion-color--` + highlight + `">` + text + `</mark>`
	}
	if href != "" {
		text = `<a` + attr("href", href) + ` rel="noopener noreferrer">` + text + `</a>`
	}
	return text
}

func externalLinkMentionHTML(value any, rawText string) string {
	href, label := externalLinkMentionValue(value)
	return linkMentionHTML(href, firstNonEmpty(mentionRawTextLabel(label), rawText))
}

func externalLinkMentionValue(value any) (string, string) {
	var href string
	var label string
	switch typed := value.(type) {
	case []any:
		if len(typed) > 0 {
			href = stringValue(typed[0])
		}
		if len(typed) > 1 {
			label = stringValue(typed[1])
		}
	case []string:
		if len(typed) > 0 {
			href = typed[0]
		}
		if len(typed) > 1 {
			label = typed[1]
		}
	case string:
		href = typed
	}
	return href, label
}

func linkMentionHTML(rawHref string, rawText string) string {
	href := safeRichTextLinkURL(rawHref)
	label := firstNonEmpty(mentionRawTextLabel(rawText), linkHostLabel(href), "Link")
	if href == "" {
		return notionMentionHTML("Link", label)
	}
	return `<a class="notion-mention notion-mention--link"` + attr("href", href) + ` rel="noopener noreferrer">` + escapeText(label) + `</a>`
}

type linkMentionMetadata struct {
	Href        string
	Title       string
	IconURL     string
	Provider    string
	Description string
	Thumbnail   string
}

func linkMentionHTMLValue(value any, rawText string, input *RenderInput, rm *recordMap) string {
	if raw, ok := value.(string); ok {
		return linkMentionHTML(raw, rawText)
	}
	meta := linkMentionMetadataFromValue(value)
	if meta.Href == "" {
		return notionMentionHTML("Link", rawText)
	}
	href := safeRichTextLinkURL(meta.Href)
	label := firstNonEmpty(mentionRawTextLabel(meta.Title), mentionRawTextLabel(rawText), linkHostLabel(href), mentionRawTextLabel(meta.Provider), "Link")
	if href == "" {
		return notionMentionHTML("Link", label)
	}
	provider := mentionRawTextLabel(meta.Provider)
	icon := safeMentionImageURL(meta.IconURL, input, rm)
	var b strings.Builder
	b.WriteString(`<span class="notion-link-mention"><a class="notion-link-mention__link"`)
	b.WriteString(attr("href", href))
	b.WriteString(` rel="noopener noreferrer">`)
	if icon != "" {
		b.WriteString(`<img class="notion-link-mention__icon"`)
		b.WriteString(attr("src", icon))
		b.WriteString(` loading="lazy" decoding="async" alt="">`)
	}
	if provider != "" {
		b.WriteString(`<span class="notion-link-mention__provider">`)
		b.WriteString(html.EscapeString(provider))
		b.WriteString(`</span>`)
	}
	b.WriteString(`<span class="notion-link-mention__title">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</span></a></span>`)
	return b.String()
}

func linkMentionMetadataFromValue(value any) linkMentionMetadata {
	data, ok := value.(map[string]any)
	if !ok {
		return linkMentionMetadata{}
	}
	return linkMentionMetadata{
		Href:        stringValue(firstNonNil(data["href"], data["url"])),
		Title:       stringValue(data["title"]),
		IconURL:     stringValue(firstNonNil(data["icon_url"], data["icon"])),
		Provider:    stringValue(firstNonNil(data["link_provider"], data["provider"])),
		Description: stringValue(data["description"]),
		Thumbnail:   stringValue(data["thumbnail_url"]),
	}
}

func safeMentionImageURL(raw string, input *RenderInput, rm *recordMap) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if input != nil {
		if local := safeURL(assetAliasURL(*input, raw)); local != "" {
			return local
		}
		if rm != nil {
			if src := unsafeNotionAssetURL(*input, *rm, raw); src != "" {
				return src
			}
		}
	}
	return safeExternalImageURL(raw)
}

func customEmojiHTML(id string, rawText string, rm recordMap, input RenderInput) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	src := strings.TrimSpace(rm.CustomEmojis[id])
	if src == "" {
		src = strings.TrimSpace(rm.CustomEmojis[NormalizeID(id)])
	}
	if src == "" {
		return ""
	}
	renderSrc := firstNonEmpty(
		resolvedAssetAliasURL(input, rm, "custom_emoji:"+id),
		resolvedAssetAliasURL(input, rm, "custom_emoji:"+NormalizeID(id)),
		resolvedAssetAliasURL(input, rm, src),
		safeExternalImageURL(src),
	)
	if renderSrc == "" {
		return ""
	}
	alt := firstNonEmpty(mentionRawTextLabel(rawText), "Custom emoji")
	return `<img class="notion-custom-emoji"` + attr("src", renderSrc) + ` loading="lazy" decoding="async"` + attr("alt", alt) + `>`
}

func resolvedMentionHTML(resolver mentionResolver, kind string, item []any, rawText string, fallback string) string {
	if resolver != nil && len(item) > 1 {
		if rendered := resolver(kind, item[1], rawText); rendered != "" {
			return rendered
		}
	}
	return notionMentionHTML(fallback, rawText)
}

func notionMentionResolver(rm recordMap, input RenderInput) mentionResolver {
	return func(kind string, value any, rawText string) string {
		switch kind {
		case "a":
			id := stringValue(value)
			anchor := notionLinkFragmentAnchor(id)
			pageID, ok := notionPageIDFromLink(id)
			if !ok {
				// Pure in-page fragment link ("#<blockid>") with no page path.
				if anchor != "" {
					return "#" + anchor
				}
				return ""
			}
			href := notionPageHrefForInput(input, pageID)
			if href == "" {
				// Target not in PagePaths; a same-page link still jumps via the
				// fragment alone.
				if anchor != "" && NormalizeID(pageID) == NormalizeID(input.PageID) {
					return "#" + anchor
				}
				return ""
			}
			if anchor != "" {
				return href + "#" + anchor
			}
			return href
		case "eoi":
			id := stringValue(value)
			blk, ok := rm.Block[NormalizeID(id)]
			if !ok || blk.Type != "external_object_instance" {
				return ""
			}
			label := firstNonEmpty(externalObjectTitle(blk), "External object")
			href := externalObjectURL(blk)
			if href == "" {
				return `<span class="notion-mention">` + html.EscapeString(label) + `</span>`
			}
			return externalObjectMentionHTML(blk, href, rawText)
		case "p":
			id := stringValue(value)
			pageID, ok := normalizedPageID(id)
			if !ok {
				return ""
			}
			label := firstNonEmpty(plainText(rm.Block[pageID].Properties["title"]), mentionRawTextLabel(rawText), "Page")
			if href := notionPageHrefForInput(input, pageID); href != "" {
				return `<a class="notion-mention notion-mention--page"` + attr("href", href) + `>` + escapeText(label) + `</a>`
			}
			return `<span class="notion-mention">` + html.EscapeString(label) + `</span>`
		case "lm":
			return linkMentionHTMLValue(value, rawText, &input, &rm)
		case "ce":
			return customEmojiHTML(stringValue(value), rawText, rm, input)
		default:
			return ""
		}
	}
}

func externalObjectMentionHTML(blk block, href string, rawText string) string {
	provider := externalObjectProvider(blk, href)
	label := firstNonEmpty(mentionRawTextLabel(rawText), externalObjectTitle(blk), provider.DefaultTitle)
	owner := externalObjectOwner(blk)
	var b strings.Builder
	b.WriteString(`<a class="notion-external-object-mention notion-external-object-mention--`)
	b.WriteString(html.EscapeString(provider.Class))
	b.WriteString(`"`)
	b.WriteString(attr("href", href))
	b.WriteString(` rel="noopener noreferrer"><span class="notion-external-object-mention__mark" aria-hidden="true">`)
	b.WriteString(html.EscapeString(provider.Mark))
	b.WriteString(`</span><span class="notion-external-object-mention__title">`)
	b.WriteString(html.EscapeString(label))
	b.WriteString(`</span>`)
	if owner != "" {
		b.WriteString(`<span class="notion-external-object-mention__meta">`)
		b.WriteString(html.EscapeString(owner))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</a>`)
	return b.String()
}

func mentionRawTextLabel(rawText string) string {
	label := strings.TrimSpace(rawText)
	if label == "" || label == "‣" {
		return ""
	}
	return label
}

func externalObjectURL(blk block) string {
	return safeURL(firstNonEmpty(
		stringValue(blk.Format["original_url"]),
		stringValue(blk.Format["uri"]),
		blockSource(blk),
	))
}

func externalObjectIsGitHub(blk block, href string) bool {
	domain := strings.TrimPrefix(strings.ToLower(stringValue(blk.Format["domain"])), "www.")
	if domain != "" && domain != "github.com" {
		return false
	}
	parsed, err := url.Parse(href)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	return host == "github.com"
}

type externalObjectProviderInfo struct {
	Class        string
	Label        string
	Mark         string
	DefaultTitle string
}

func externalObjectProvider(blk block, href string) externalObjectProviderInfo {
	if externalObjectIsGitHub(blk, href) {
		return externalObjectProviderInfo{
			Class:        "github",
			Label:        "GitHub",
			Mark:         "GH",
			DefaultTitle: "GitHub item",
		}
	}
	label := firstNonEmpty(linkHostLabel(href), "External")
	class := sanitizeClassToken(strings.ReplaceAll(label, ".", "-"))
	if class == "" {
		class = "external"
	}
	return externalObjectProviderInfo{
		Class:        class,
		Label:        label,
		Mark:         "EX",
		DefaultTitle: "External object",
	}
}

func externalObjectTitle(blk block) string {
	return externalObjectAttributeText(blk, "title", "name")
}

func externalObjectOwner(blk block) string {
	owner := externalObjectAttributeText(blk, "owner")
	if owner == "" {
		return ""
	}
	if parsed, err := url.Parse(owner); err == nil && parsed.Host != "" {
		segments := pathSegments(parsed.Path)
		if len(segments) > 0 {
			return segments[len(segments)-1]
		}
	}
	return owner
}

func externalObjectUpdatedAt(blk block) string {
	for _, raw := range externalObjectAttributeValues(blk, "updated_at") {
		if label, _ := formatDateMention(raw); label != "" {
			return label
		}
		if label := stringValue(raw); label != "" {
			return label
		}
	}
	return ""
}

func externalObjectVisualURL(input RenderInput, rm recordMap, blk block) string {
	if src := resolvedAssetURL(input, rm, blk.ID, "avatar_url", ""); src != "" {
		return src
	}
	if src := resolvedAssetURL(input, rm, blk.ID, "icon", externalObjectIconRawURL(blk)); src != "" {
		return src
	}
	return externalObjectIconURL(blk)
}

func externalObjectIconRawURL(blk block) string {
	for _, raw := range externalObjectAttributes(blk) {
		attr, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := strings.ToLower(stringValue(attr["id"]))
		mimeType := strings.ToLower(stringValue(attr["mimeType"]))
		formatType := ""
		if format, ok := attr["format"].(map[string]any); ok {
			formatType = strings.ToLower(stringValue(format["type"]))
		}
		if id != "avatar_url" && formatType != "icon" && !strings.HasPrefix(mimeType, "image/") {
			continue
		}
		values, ok := attr["values"].([]any)
		if !ok || len(values) == 0 {
			continue
		}
		return stringValue(values[0])
	}
	return ""
}

func externalObjectIconURL(blk block) string {
	if src := safeExternalImageURL(externalObjectIconRawURL(blk)); src != "" {
		return src
	}
	return ""
}

func externalObjectAttributeText(blk block, keys ...string) string {
	for _, raw := range externalObjectAttributeValues(blk, keys...) {
		if label, _ := formatDateMention(raw); label != "" {
			return label
		}
		if label := stringValue(raw); label != "" {
			return label
		}
	}
	return ""
}

func externalObjectAttributeValues(blk block, keys ...string) []any {
	keySet := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key != "" {
			keySet[key] = struct{}{}
		}
	}
	for _, raw := range externalObjectAttributes(blk) {
		attr, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := strings.ToLower(stringValue(attr["id"]))
		name := strings.ToLower(stringValue(attr["name"]))
		if _, ok := keySet[id]; !ok {
			if _, ok := keySet[name]; !ok {
				continue
			}
		}
		values, ok := attr["values"].([]any)
		if !ok || len(values) == 0 {
			return nil
		}
		return values
	}
	return nil
}

func externalObjectAttributes(blk block) []any {
	attrs, ok := blk.Format["attributes"].([]any)
	if !ok {
		return nil
	}
	return attrs
}

func equationText(value any) string {
	raw, ok := value.([]any)
	if !ok {
		return cleanEquationText(stringValue(value))
	}
	var fallback strings.Builder
	for _, part := range raw {
		arr, ok := part.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		if text, ok := arr[0].(string); ok {
			fallback.WriteString(text)
		}
		if len(arr) < 2 {
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
			if kind != "e" {
				continue
			}
			if equation := stringValue(item[1]); equation != "" {
				return cleanEquationText(equation)
			}
		}
	}
	return cleanEquationText(fallback.String())
}

func cleanEquationText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.Contains(value, `\`) {
		return stripEquationDelimiters(value)
	}
	value = stripEquationDelimiters(value)
	if idx := strings.Index(value, `<1.1+(`); idx > 0 && strings.Contains(value[:idx], `\lvert`) && containsPlainMathUnicode(value[idx:]) {
		return strings.TrimSpace(value[:idx] + `<1.`)
	}
	if idx := latexPlainTextTailIndex(value); idx > 0 {
		return strings.TrimSpace(value[:idx])
	}
	return value
}

func stripEquationDelimiters(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	pairs := [][2]string{
		{`\[`, `\]`},
		{`\(`, `\)`},
		{`$$`, `$$`},
		{`$`, `$`},
	}
	for _, pair := range pairs {
		if strings.HasPrefix(value, pair[0]) && strings.HasSuffix(value, pair[1]) && len(value) > len(pair[0])+len(pair[1]) {
			return strings.TrimSpace(value[len(pair[0]) : len(value)-len(pair[1])])
		}
	}
	if strings.HasPrefix(value, `[`) && strings.HasSuffix(value, `]`) && likelyDisplayEquationDelimiter(value) {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func likelyDisplayEquationDelimiter(value string) bool {
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, `[`), `]`))
	if inner == "" {
		return false
	}
	if strings.HasPrefix(value, `[ `) || strings.HasSuffix(value, ` ]`) {
		return true
	}
	return strings.ContainsAny(inner, `\^_=`)
}

func latexPlainTextTailIndex(value string) int {
	for _, sep := range []string{`}(`, `)(`} {
		offset := 0
		for {
			idx := strings.Index(value[offset:], sep)
			if idx < 0 {
				break
			}
			idx += offset
			prefix := value[:idx+1]
			suffix := value[idx+1:]
			if strings.Contains(prefix, `\`) && containsPlainMathUnicodePrefix(suffix, 32) {
				return idx + 1
			}
			offset = idx + len(sep)
		}
	}
	return -1
}

func containsPlainMathUnicode(value string) bool {
	return strings.ContainsAny(value, "ϕ∑∏∣≤≥≠−⋯")
}

func containsPlainMathUnicodePrefix(value string, limit int) bool {
	if limit <= 0 {
		return false
	}
	i := 0
	for end := range value {
		if i >= limit {
			return containsPlainMathUnicode(value[:end])
		}
		i++
	}
	return containsPlainMathUnicode(value)
}

func notionMentionHTML(fallback string, rawText string) string {
	label := strings.TrimSpace(rawText)
	if label == "" || label == "‣" {
		label = fallback
	}
	if fallback == "Person" && !strings.HasPrefix(label, "@") {
		label = "@" + label
	}
	return `<span class="notion-mention">` + html.EscapeString(label) + `</span>`
}

func notionColorClass(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	background := strings.HasSuffix(value, "_background")
	base := strings.TrimSuffix(value, "_background")
	switch base {
	case "gray", "brown", "orange", "yellow", "green", "blue", "purple", "pink", "red", "teal":
		if background {
			return base + "-background"
		}
		return base
	default:
		return ""
	}
}

func notionBlockColorClass(blk block) string {
	color, _ := blk.Format["block_color"].(string)
	color = strings.ToLower(strings.TrimSpace(color))
	if color == "" || color == "default" {
		return ""
	}
	background := strings.HasSuffix(color, "_background")
	base := notionColorClass(strings.TrimSuffix(color, "_background"))
	if base == "" {
		return ""
	}
	if background {
		return "notion-color--" + base + "-background"
	}
	return "notion-color--" + base
}

func sanitizeClassToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			b.WriteRune(ch)
		case ch == '-' || ch == '_' || ch == '+':
			b.WriteRune(ch)
		case ch == ' ':
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-_+")
}

func blockSource(blk block) string {
	for _, value := range []string{
		plainText(blk.Properties["source"]),
		plainText(blk.Properties["link"]),
		stringValue(blk.Format["source"]),
		stringValue(blk.Format["display_source"]),
		stringValue(blk.Format["original_url"]),
		stringValue(blk.Format["uri"]),
		formatNestedString(blk.Format["drive_properties"], "url"),
	} {
		if safe := safeURL(value); safe != "" {
			return safe
		}
	}
	return ""
}

func formatNestedString(value any, key string) string {
	data, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(data[key])
}
