package notion

import (
	"html"
	"strings"
)

func renderFileLink(out *strings.Builder, src string, label string) {
	out.WriteString(`<p class="notion-file"><a`)
	out.WriteString(attr("href", src))
	if !strings.HasPrefix(src, "/") {
		out.WriteString(` rel="noopener noreferrer"`)
	}
	out.WriteString(`><span class="notion-file__icon" aria-hidden="true"></span><span class="notion-file__name">`)
	out.WriteString(label)
	out.WriteString(`</span></a></p>`)
}

func renderFilePlaceholder(out *strings.Builder, label string) {
	out.WriteString(`<p class="notion-file notion-file--unavailable"><span class="notion-file__placeholder"><span class="notion-file__icon" aria-hidden="true"></span><span class="notion-file__name">`)
	out.WriteString(label)
	out.WriteString(`</span></span></p>`)
}

func renderPDFEmbed(out *strings.Builder, src string, label string, input RenderInput) {
	iframeTitle := firstNonEmpty(strings.TrimSpace(pdfFilename(src)), input.t("notion.pdf", "PDF document"))
	out.WriteString(`<figure class="notion-embed notion-embed--pdf"><iframe`)
	out.WriteString(attr("title", iframeTitle))
	out.WriteString(attr("src", src))
	out.WriteString(` loading="lazy" referrerpolicy="strict-origin-when-cross-origin" sandbox="allow-same-origin"></iframe><figcaption><a`)
	out.WriteString(attr("href", src))
	out.WriteString(`>`)
	out.WriteString(label)
	out.WriteString(`</a></figcaption></figure>`)
}

func renderEmbedFigcaption(out *strings.Builder, rm recordMap, blk block, href string, label string, input RenderInput) {
	caption := richTextWithResolver(blk.Properties["caption"], notionMentionResolver(rm, input))
	if caption == "" && href == "" {
		return
	}
	out.WriteString(`<figcaption>`)
	if caption != "" {
		out.WriteString(caption)
		if href != "" {
			out.WriteString(`<br>`)
		}
	}
	if href != "" {
		out.WriteString(`<a`)
		out.WriteString(attr("href", href))
		out.WriteString(` rel="noopener noreferrer">`)
		out.WriteString(label)
		out.WriteString(`</a>`)
	}
	out.WriteString(`</figcaption>`)
}

func renderExternalLinkBlock(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) {
	href := blockSource(blk)
	if href == "" {
		return
	}
	externalTitle := externalObjectTitle(blk)
	if externalTitle != "" {
		externalTitle = html.EscapeString(externalTitle)
	}
	label := firstText(firstNonEmpty(text, externalTitle), href)
	description := richText(blk.Properties["description"])
	iconURL := firstNonEmpty(bookmarkVisualURL(input, rm, blk, "bookmark_icon", "icon"), externalObjectIconURL(blk))
	coverURL := bookmarkVisualURL(input, rm, blk, "bookmark_cover", "cover")
	out.WriteString(`<figure class="notion-bookmark`)
	if blockType := sanitizeClassToken(blk.Type); blockType != "" {
		out.WriteString(` notion-bookmark--`)
		out.WriteString(html.EscapeString(blockType))
	}
	if coverURL != "" {
		out.WriteString(` notion-bookmark--with-cover`)
	}
	out.WriteString(`"><a`)
	out.WriteString(attr("href", href))
	out.WriteString(` rel="noopener noreferrer"><span class="notion-bookmark__body">`)
	if iconURL != "" {
		out.WriteString(`<span class="notion-bookmark__title-row"><img class="notion-bookmark__icon"`)
		out.WriteString(attr("src", iconURL))
		out.WriteString(` loading="lazy" decoding="async" alt=""><strong>`)
		out.WriteString(label)
		out.WriteString(`</strong></span>`)
	} else {
		out.WriteString(`<strong>`)
		out.WriteString(label)
		out.WriteString(`</strong>`)
	}
	if description != "" {
		out.WriteString(`<span class="notion-bookmark__description">`)
		out.WriteString(description)
		out.WriteString(`</span>`)
	}
	if host := linkHostLabel(href); host != "" {
		out.WriteString(`<span class="notion-bookmark__host">`)
		out.WriteString(html.EscapeString(host))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</span>`)
	if coverURL != "" {
		out.WriteString(`<span class="notion-bookmark__cover"><img`)
		out.WriteString(attr("src", coverURL))
		out.WriteString(` loading="lazy" decoding="async" alt=""></span>`)
	}
	out.WriteString(`</a></figure>`)
}

func renderDriveCard(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) bool {
	props, ok := blk.Format["drive_properties"].(map[string]any)
	if !ok {
		return false
	}
	href := safeURL(stringValue(props["url"]))
	if href == "" {
		return false
	}
	title := firstText(firstNonEmpty(text, html.EscapeString(stringValue(props["title"]))), "Google Drive document")
	thumbnail := driveVisualURL(input, rm, blk, props, "thumbnail", "drive-thumbnail")
	icon := driveVisualURL(input, rm, blk, props, "icon", "drive-icon")
	domain := linkHostLabel(href)
	out.WriteString(`<figure class="notion-google-drive"><a class="notion-google-drive__link"`)
	out.WriteString(attr("href", href))
	out.WriteString(` rel="noopener noreferrer">`)
	out.WriteString(`<span class="notion-google-drive__preview">`)
	if thumbnail != "" {
		out.WriteString(`<img`)
		out.WriteString(attr("src", thumbnail))
		out.WriteString(` loading="lazy" decoding="async" alt="">`)
	} else {
		out.WriteString(`<span class="notion-google-drive__placeholder" aria-hidden="true"></span>`)
	}
	out.WriteString(`</span><span class="notion-google-drive__body"><strong>`)
	out.WriteString(title)
	out.WriteString(`</strong>`)
	if domain != "" || icon != "" {
		out.WriteString(`<span class="notion-google-drive__source">`)
		if icon != "" {
			out.WriteString(`<img class="notion-google-drive__icon"`)
			out.WriteString(attr("src", icon))
			out.WriteString(` loading="lazy" decoding="async" alt="">`)
		}
		if domain != "" {
			out.WriteString(`<span>`)
			out.WriteString(html.EscapeString(domain))
			out.WriteString(`</span>`)
		}
		out.WriteString(`</span>`)
	}
	out.WriteString(`</span></a></figure>`)
	return true
}

func renderExternalObjectBlock(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) bool {
	href := externalObjectURL(blk)
	if href == "" {
		return false
	}
	title := externalObjectTitle(blk)
	provider := externalObjectProvider(blk, href)
	titleHTML := firstText(firstNonEmpty(text, html.EscapeString(title)), provider.DefaultTitle)
	owner := externalObjectOwner(blk)
	updated := externalObjectUpdatedAt(blk)
	iconURL := externalObjectVisualURL(input, rm, blk)
	out.WriteString(`<figure class="notion-external-object notion-external-object--`)
	out.WriteString(html.EscapeString(provider.Class))
	out.WriteString(`"><a class="notion-external-object__link"`)
	out.WriteString(attr("href", href))
	out.WriteString(` rel="noopener noreferrer">`)
	out.WriteString(`<span class="notion-external-object__icon" aria-hidden="true">`)
	if iconURL != "" {
		out.WriteString(`<img`)
		out.WriteString(attr("src", iconURL))
		out.WriteString(` loading="lazy" decoding="async" alt="">`)
	} else {
		out.WriteString(`<span>`)
		out.WriteString(html.EscapeString(provider.Mark))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</span><span class="notion-external-object__body"><span class="notion-external-object__eyebrow">`)
	out.WriteString(html.EscapeString(provider.Label))
	out.WriteString(`</span><strong class="notion-external-object__title">`)
	out.WriteString(titleHTML)
	out.WriteString(`</strong>`)
	if owner != "" || updated != "" {
		out.WriteString(`<span class="notion-external-object__meta">`)
		if owner != "" {
			out.WriteString(html.EscapeString(owner))
		}
		if owner != "" && updated != "" {
			out.WriteString(` · `)
		}
		if updated != "" {
			out.WriteString(html.EscapeString(updated))
		}
		out.WriteString(`</span>`)
	}
	out.WriteString(`</span></a></figure>`)
	return true
}

func driveVisualURL(input RenderInput, rm recordMap, blk block, props map[string]any, key string, role string) string {
	raw := stringValue(props[key])
	if raw == "" {
		return ""
	}
	if src := resolvedAssetURL(input, rm, blk.ID, role, raw); src != "" {
		return src
	}
	return safeExternalImageURL(raw)
}

func bookmarkVisualURL(input RenderInput, rm recordMap, blk block, formatKey string, role string) string {
	raw := stringValue(blk.Format[formatKey])
	if src := resolvedAssetURL(input, rm, blk.ID, role, raw); src != "" {
		return src
	}
	src := safeURL(raw)
	if src == "" || isNotionHostedURL(src) {
		return ""
	}
	return src
}

func renderEmbeddedOrExternalLinkBlock(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) {
	href := blockSource(blk)
	if href == "" {
		return
	}
	if embed := safeEmbedURL(blk.Type, href); embed != "" {
		label := firstText(text, href)
		out.WriteString(`<figure class="`)
		out.WriteString(html.EscapeString(embedClass(blk.Type, embed)))
		if style := embedStyle(blk); style != "" {
			out.WriteString(`" style="`)
			out.WriteString(style)
		}
		iframeTitle := firstNonEmpty(linkHostLabel(href), input.t("notion.embed", "Embedded content"))
		out.WriteString(`"><iframe`)
		out.WriteString(attr("title", iframeTitle))
		out.WriteString(attr("src", embed))
		out.WriteString(` loading="lazy" referrerpolicy="strict-origin-when-cross-origin" sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-presentation" allowfullscreen></iframe>`)
		renderEmbedFigcaption(out, rm, blk, href, label, input)
		out.WriteString(`</figure>`)
		return
	}
	renderExternalLinkBlock(out, rm, blk, text, input)
}

func renderTweetBlock(out *strings.Builder, rm recordMap, blk block, text string, input RenderInput) {
	href := blockSource(blk)
	if href == "" {
		return
	}
	info, ok := parseTweetURL(href)
	if !ok {
		renderExternalLinkBlock(out, rm, blk, text, input)
		return
	}
	tweetLabel := input.t("notion.tweet", "Tweet")
	label := tweetLabel
	if info.User != "" {
		label = tweetLabel + " by @" + info.User
	}
	out.WriteString(`<figure class="notion-tweet"><a`)
	out.WriteString(attr("href", href))
	out.WriteString(` rel="noopener noreferrer"><span class="notion-tweet__eyebrow">`)
	out.WriteString(html.EscapeString(tweetLabel))
	out.WriteString(`</span><strong>`)
	out.WriteString(firstText(text, label))
	out.WriteString(`</strong>`)
	if info.ID != "" {
		out.WriteString(`<span class="notion-tweet__meta">`)
		out.WriteString(html.EscapeString(tweetHostLabel(href)))
		out.WriteString(` · `)
		out.WriteString(html.EscapeString(info.ID))
		out.WriteString(`</span>`)
	}
	out.WriteString(`</a></figure>`)
}

func renderLightboxImage(out *strings.Builder, className string, src string, alt string) {
	escapedSrc := html.EscapeString(src)
	out.WriteString(`<a class="`)
	out.WriteString(html.EscapeString(className))
	out.WriteString(`" href="`)
	out.WriteString(escapedSrc)
	out.WriteString(`" data-collection-lightbox data-full-src="`)
	out.WriteString(escapedSrc)
	out.WriteString(`"><img src="`)
	out.WriteString(escapedSrc)
	out.WriteString(`" loading="lazy" decoding="async" alt="`)
	out.WriteString(html.EscapeString(alt))
	out.WriteString(`"></a>`)
}
