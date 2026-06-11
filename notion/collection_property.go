package notion

import (
	"fmt"
	"html"
	"math"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

func visibleCollectionProperties(view collectionView, coll collection) []string {
	specs := visibleCollectionPropertySpecs(view, coll)
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		out = append(out, spec.Property)
	}
	return out
}

func visibleCollectionPropertySpecs(view collectionView, coll collection) []collectionViewProperty {
	key := view.Type + "_properties"
	raw, ok := view.Format[key].([]any)
	if !ok && view.Type == "board" {
		raw, ok = view.Format["board_properties"].([]any)
	}
	out := make([]collectionViewProperty, 0)
	if ok {
		for _, item := range raw {
			data, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if visible, ok := data["visible"].(bool); ok && !visible {
				continue
			}
			property, _ := data["property"].(string)
			if _, ok := coll.Schema[property]; property != "" && ok {
				out = append(out, collectionViewProperty{
					Property: property,
					Width:    collectionViewPropertyWidth(data["width"], coll.Schema[property]),
				})
			}
		}
	}
	if len(out) == 0 {
		// No view-configured columns: fall back to the title plus up to five
		// other schema properties. Iterate the schema in a stable sorted order
		// so the column set and ordering are deterministic across renders
		// (Go map iteration order is randomized).
		out = append(out, collectionViewProperty{Property: "title", Width: collectionViewPropertyWidth(nil, coll.Schema["title"])})
		others := make([]string, 0, len(coll.Schema))
		for property := range coll.Schema {
			if property != "title" {
				others = append(others, property)
			}
		}
		sort.Strings(others)
		for _, property := range others {
			if len(out) >= 6 {
				break
			}
			out = append(out, collectionViewProperty{Property: property, Width: collectionViewPropertyWidth(nil, coll.Schema[property])})
		}
	}
	return dedupeCollectionViewProperties(out)
}

func collectionViewPropertyWidth(value any, schema collectionProperty) int {
	if width, ok := numberValue(value); ok && !math.IsNaN(width) && !math.IsInf(width, 0) {
		return clampInt(int(math.Round(width)), 60, 600)
	}
	switch schema.Type {
	case "title":
		return 280
	default:
		return 200
	}
}

func dedupeCollectionViewProperties(specs []collectionViewProperty) []collectionViewProperty {
	seen := map[string]bool{}
	out := make([]collectionViewProperty, 0, len(specs))
	for _, spec := range specs {
		if spec.Property == "" || seen[spec.Property] {
			continue
		}
		seen[spec.Property] = true
		if spec.Width <= 0 {
			spec.Width = 160
		}
		out = append(out, spec)
	}
	return out
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func collectionPropertyHTML(rm recordMap, row block, coll collection, property string, input RenderInput) string {
	value := row.Properties[property]
	schema := collectionProperty{}
	if current, ok := coll.Schema[property]; ok {
		schema = current
	}
	if collectionPropertyValueEmpty(value) {
		switch schema.Type {
		case "created_time":
			value = blockTimestampValue(row.CreatedTime)
		case "last_edited_time":
			value = blockTimestampValue(row.LastEditedTime)
		}
	}
	switch schema.Type {
	case "title":
		title := richText(value)
		icon := pageIconHTML(rm, row, input)
		if pageID, ok := normalizedPageID(row.ID); ok {
			if href := notionPageHrefForInput(input, pageID); href != "" {
				return `<a class="notion-collection-title"` + attr("href", href) + `>` + icon + firstText(title, input.t("notion.untitled", "Untitled")) + `</a>`
			}
		}
		return icon + title
	case "select", "multi_select", "status":
		return renderCollectionPills(collectionPlainValues(value), schema)
	case "checkbox":
		return renderCollectionCheckbox(plainText(value))
	case "number":
		return renderCollectionNumber(plainText(value), schema.NumberFormat)
	case "date", "created_time", "last_edited_time":
		return renderCollectionDate(value)
	case "person", "created_by", "last_edited_by":
		return renderCollectionPeople(value)
	case "relation":
		return renderCollectionRelation(value, rm, input)
	case "formula":
		if collectionPropertyValueEmpty(value) && len(schema.Formula) > 0 {
			if formulaValue, ok := evalCollectionFormula(schema.Formula, row, coll); ok {
				return renderCollectionFormulaResult(formulaValue, schema, rm, input)
			}
		}
		return renderCollectionComputedProperty(value, schema, rm, input)
	case "rollup":
		return renderCollectionComputedProperty(value, schema, rm, input)
	case "file", "files":
		return renderCollectionFiles(value, rm, input)
	case "url":
		return renderCollectionURL(plainText(value))
	case "email":
		return renderEmailLink(plainText(value))
	case "phone_number":
		return renderPhoneLink(plainText(value))
	case "unique_id":
		return renderCollectionUniqueID(plainText(value), schema)
	default:
		return richTextWithResolver(value, notionMentionResolver(rm, input))
	}
}

// renderCollectionUniqueID renders a unique ID value as Notion shows it:
// PREFIX-N when the schema carries a prefix the value lacks, otherwise the
// value as-is.
func renderCollectionUniqueID(text string, schema collectionProperty) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if schema.Prefix != "" && !strings.HasPrefix(text, schema.Prefix+"-") {
		text = schema.Prefix + "-" + text
	}
	return `<span class="notion-property-unique-id">` + html.EscapeString(text) + `</span>`
}

func collectionPropertyValueEmpty(value any) bool {
	if value == nil {
		return true
	}
	return strings.TrimSpace(plainText(value)) == ""
}

func blockTimestampValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return formatUnixTimestamp(typed)
	case int64:
		return formatUnixTimestamp(float64(typed))
	case int:
		return formatUnixTimestamp(float64(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}

func formatUnixTimestamp(value float64) string {
	if value <= 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return ""
	}
	if value > 1_000_000_000_000 {
		return time.UnixMilli(int64(value)).UTC().Format(time.RFC3339)
	}
	return time.Unix(int64(value), 0).UTC().Format(time.RFC3339)
}

func collectionCoverHTML(rm recordMap, row block, input RenderInput) string {
	cover := stringValue(row.Format["page_cover"])
	if src := resolvedAssetURL(input, rm, row.ID, "cover", cover); src != "" {
		var b strings.Builder
		b.WriteString(`<div class="notion-collection-card__cover">`)
		renderLightboxImage(&b, "notion-collection-card__cover-link", src, "")
		b.WriteString(`</div>`)
		return b.String()
	}
	if icon := pageIconHTML(rm, row, input); icon != "" {
		return `<div class="notion-collection-card__cover notion-collection-card__cover--icon">` + icon + `</div>`
	}
	return ""
}

func collectionPropertyName(coll collection, property string) string {
	if property == "title" {
		return "Name"
	}
	if schema, ok := coll.Schema[property]; ok && strings.TrimSpace(schema.Name) != "" {
		return schema.Name
	}
	return property
}

func renderCollectionPills(labels []string, schemas ...collectionProperty) string {
	if len(labels) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<span class="notion-property-pills">`)
	for _, label := range labels {
		classes := []string{"notion-property-pill"}
		if colorClass := collectionPillColorClass(label, schemas...); colorClass != "" {
			classes = append(classes, colorClass)
		}
		b.WriteString(`<span class="`)
		b.WriteString(html.EscapeString(strings.Join(classes, " ")))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(label))
		b.WriteString(`</span>`)
	}
	b.WriteString(`</span>`)
	return b.String()
}

func collectionPillColorClass(label string, schemas ...collectionProperty) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return ""
	}
	lower := strings.ToLower(label)
	for _, schema := range schemas {
		if len(schema.OptionColors) == 0 {
			continue
		}
		if className := schema.OptionColors[label]; className != "" {
			return className
		}
		if className := schema.OptionColors[lower]; className != "" {
			return className
		}
	}
	return ""
}

func collectionPlainValues(value any) []string {
	return collectionPlainValuesFromText(plainText(value))
}

func collectionPlainValuesFromText(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 && strings.TrimSpace(text) != "" {
		out = append(out, strings.TrimSpace(text))
	}
	return out
}

func renderCollectionCheckbox(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	checked := normalized == "yes" || normalized == "true" || normalized == "checked" || normalized == "1"
	label := "Unchecked"
	if checked {
		label = "Checked"
	}
	return `<span class="notion-property-checkbox"><input type="checkbox" disabled aria-label="` + label + `"` + checkedAttr(checked) + `></span>`
}

func checkedAttr(checked bool) string {
	if checked {
		return " checked"
	}
	return ""
}

func renderCollectionURL(value string) string {
	value = strings.TrimSpace(value)
	if href := safeAttrURL(value); href != "" {
		return `<a href="` + href + `" rel="noopener noreferrer">` + html.EscapeString(value) + `</a>`
	}
	return html.EscapeString(value)
}

func renderEmailLink(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.Contains(value, "@") && !strings.ContainsAny(value, " \t\r\n\"'<>") {
		return `<a href="mailto:` + html.EscapeString(value) + `">` + html.EscapeString(value) + `</a>`
	}
	return html.EscapeString(value)
}

func renderPhoneLink(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var number strings.Builder
	for _, ch := range value {
		if (ch >= '0' && ch <= '9') || ch == '+' {
			number.WriteRune(ch)
		}
	}
	if number.Len() == 0 {
		return html.EscapeString(value)
	}
	return `<a href="tel:` + html.EscapeString(number.String()) + `">` + html.EscapeString(value) + `</a>`
}

func renderCollectionNumber(value string, format string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", ""), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
		return html.EscapeString(value)
	}
	return `<span class="notion-property-number">` + html.EscapeString(formatCollectionNumber(parsed, format)) + `</span>`
}

func formatCollectionNumber(value float64, format string) string {
	format = strings.ToLower(strings.TrimSpace(format))
	negative := value < 0
	absValue := math.Abs(value)
	number := formatFloat(absValue)
	baseFormat := strings.TrimSuffix(format, "_with_commas")
	switch format {
	case "number_with_commas":
		number = addCommas(number)
	case "percent":
		// Round the scaled value before formatting: multiplying a stored
		// fraction by 100 in float64 introduces representation noise (0.07*100
		// -> 7.000000000000001), which formatFloat would otherwise render in
		// full. Notion displays a rounded percentage.
		number = formatFloat(math.Round(absValue*100*1e6)/1e6) + "%"
	case "percent_with_commas":
		number = addCommas(formatFloat(math.Round(absValue*100*1e6)/1e6)) + "%"
	default:
		if symbol := collectionCurrencySymbol(baseFormat); symbol != "" {
			number = symbol + addCommas(formatCurrencyAmount(absValue, baseFormat))
		} else if strings.Contains(format, "comma") {
			number = addCommas(number)
		}
	}
	if negative {
		return "-" + number
	}
	return number
}

func formatCurrencyAmount(value float64, format string) string {
	precision := 2
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "chilean_peso", "colombian_peso", "indonesian_rupiah", "rupiah", "japanese_yen", "yen", "korean_won", "won":
		precision = 0
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func collectionCurrencySymbol(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "argentine_peso":
		return "ARS "
	case "australian_dollar":
		return "A$"
	case "baht":
		return "฿"
	case "brazilian_real":
		return "R$"
	case "canadian_dollar":
		return "C$"
	case "chilean_peso":
		return "CLP "
	case "colombian_peso":
		return "COP "
	case "danish_krone", "norwegian_krone", "krona":
		return "kr "
	case "dirham":
		return "AED "
	case "dollar":
		return "$"
	case "euro":
		return "€"
	case "forint":
		return "Ft "
	case "franc":
		return "Fr "
	case "hong_kong_dollar":
		return "HK$"
	case "koruna":
		return "Kč "
	case "leu":
		return "lei "
	case "lira":
		return "₺"
	case "mexican_peso":
		return "MX$"
	case "new_taiwan_dollar":
		return "NT$"
	case "new_zealand_dollar":
		return "NZ$"
	case "peruvian_sol":
		return "S/ "
	case "philippine_peso":
		return "₱"
	case "pound":
		return "£"
	case "rand":
		return "R "
	case "ringgit":
		return "RM "
	case "riyals", "riyal":
		return "SAR "
	case "rubles", "ruble":
		return "₽"
	case "rupee":
		return "₹"
	case "rupiah":
		return "Rp "
	case "shekel":
		return "₪"
	case "singapore_dollar":
		return "S$"
	case "uruguayan_peso":
		return "$U "
	case "yen":
		return "¥"
	case "yuan":
		return "CN¥"
	case "zloty":
		return "zł "
	case "won":
		return "₩"
	default:
		return ""
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func addCommas(value string) string {
	parts := strings.SplitN(value, ".", 2)
	whole := parts[0]
	if len(whole) <= 3 {
		return value
	}
	var b strings.Builder
	prefix := len(whole) % 3
	if prefix == 0 {
		prefix = 3
	}
	b.WriteString(whole[:prefix])
	for i := prefix; i < len(whole); i += 3 {
		b.WriteByte(',')
		b.WriteString(whole[i : i+3])
	}
	if len(parts) == 2 {
		b.WriteByte('.')
		b.WriteString(parts[1])
	}
	return b.String()
}

func renderCollectionDate(value any) string {
	if label, datetime, _ := formatDateRangeMention(value); label != "" {
		return `<span class="notion-date-mention"><time datetime="` + html.EscapeString(datetime) + `">` + html.EscapeString(label) + `</time></span>`
	}
	rendered := richTextWithResolver(value, nil)
	if rendered == "" {
		return ""
	}
	if strings.Contains(rendered, "notion-date-mention") {
		return rendered
	}
	text := plainText(value)
	if text == "" {
		text = strings.TrimSpace(rendered)
	}
	label, datetime := collectionDateTextLabel(text)
	if label == "" {
		return ""
	}
	return `<span class="notion-date-mention"><time datetime="` + html.EscapeString(datetime) + `">` + html.EscapeString(label) + `</time></span>`
}

func collectionDateTextLabel(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" || text == "<nil>" {
		return "", ""
	}
	if parsed, err := time.Parse(time.RFC3339, text); err == nil {
		return parsed.Format("Jan 2, 2006 3:04 PM"), parsed.Format(time.RFC3339)
	}
	if key := dateKey(text); key != "" {
		if parsed, ok := parseDateKey(key); ok {
			return parsed.Format("Jan 2, 2006"), key
		}
		return key, key
	}
	return text, text
}

func sameDayEndClockLabel(start string, end string) (bool, string) {
	startDate, _, okStart := collectionDateTimeParts(start)
	endDate, endClock, okEnd := collectionDateTimeParts(end)
	return okStart && okEnd && startDate == endDate, endClock
}

func collectionDateTimeParts(text string) (string, string, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(text))
	if err != nil {
		return "", "", false
	}
	return parsed.Format("2006-01-02"), parsed.Format("3:04 PM"), true
}

func collectionClockLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse("15:04", value); err == nil {
		return parsed.Format("3:04 PM")
	}
	if parsed, err := time.Parse("15:04:05", value); err == nil {
		return parsed.Format("3:04 PM")
	}
	if parsed, err := time.Parse("3:04 PM", strings.ToUpper(value)); err == nil {
		return parsed.Format("3:04 PM")
	}
	return value
}

func renderCollectionPeople(value any) string {
	// Computed/rollup people can arrive as a person object (map) or a list of
	// them. richTextWithResolver would stringify a bare map as raw Go text
	// ("map[...]"), so pull display names out first.
	if names, ok := computedPersonNames(value); ok {
		return renderPersonChips(names)
	}
	rendered := richTextWithResolver(value, nil)
	if rendered != "" {
		return rendered
	}
	return renderPersonChips(collectionPlainValues(value))
}

func renderPersonChips(names []string) string {
	if len(names) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<span class="notion-property-people">`)
	for _, person := range names {
		b.WriteString(notionMentionHTML("Person", person))
	}
	b.WriteString(`</span>`)
	return b.String()
}

// computedPersonNames returns (names, true) when value is a computed person
// payload — a person object map or a list whose items include person maps — and
// (nil, false) when it is a rich-text array or other shape handled elsewhere.
func computedPersonNames(value any) ([]string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		if name := strings.TrimSpace(computedPlainText(typed)); name != "" {
			return []string{name}, true
		}
		return nil, true
	case []any:
		if richTextArray(typed) {
			return nil, false
		}
		hasMap := false
		names := make([]string, 0, len(typed))
		for _, item := range typed {
			if _, ok := item.(map[string]any); ok {
				hasMap = true
			}
			if name := strings.TrimSpace(computedPlainText(item)); name != "" {
				names = append(names, name)
			}
		}
		if !hasMap {
			return nil, false
		}
		return names, true
	default:
		return nil, false
	}
}

func renderCollectionRelation(value any, rm recordMap, input RenderInput) string {
	rendered := richTextWithResolver(value, notionMentionResolver(rm, input))
	if rendered != "" {
		return rendered
	}
	return renderCollectionPills(collectionPlainValues(value))
}

func renderCollectionFiles(value any, rm recordMap, input RenderInput) string {
	files := collectionFileCandidates(value)
	if len(files) == 0 {
		if rendered := richText(value); rendered != "" {
			return rendered
		}
		return ""
	}
	var b strings.Builder
	b.WriteString(`<span class="notion-property-files">`)
	for _, file := range files {
		resolved := firstNonEmpty(
			resolvedAssetAliasURL(input, rm, file.Source),
			resolvedAssetAliasURL(input, rm, file.Label),
		)
		href := safeURL(firstNonEmpty(resolved, file.Source))
		if href == "" || (resolved == "" && isNotionHostedURL(file.Source)) {
			b.WriteString(`<span>`)
			b.WriteString(html.EscapeString(file.DisplayLabel()))
			b.WriteString(`</span>`)
			continue
		}
		b.WriteString(`<a`)
		b.WriteString(attr("href", href))
		b.WriteString(` rel="noopener noreferrer">`)
		b.WriteString(html.EscapeString(file.DisplayLabel()))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</span>`)
	return b.String()
}

type collectionFileCandidate struct {
	Label  string
	Source string
}

// DisplayLabel returns the human-readable name to show for the file candidate,
// preferring an explicit non-URL label and otherwise deriving one from the
// source URL.
func (c collectionFileCandidate) DisplayLabel() string {
	if label := strings.TrimSpace(c.Label); label != "" && label != "," {
		if safeURL(label) != "" || strings.HasPrefix(label, "attachment:") {
			return fileLabel(label)
		}
		return label
	}
	return fileLabel(c.Source)
}

func collectionFileCandidates(value any) []collectionFileCandidate {
	if items, ok := value.([]any); ok && richTextArray(items) {
		out := make([]collectionFileCandidate, 0, len(items))
		for _, item := range items {
			part, _ := item.([]any)
			label, _ := part[0].(string)
			label = strings.TrimSpace(label)
			if label == "" || label == "," {
				continue
			}
			if href := richTextLinkDecoration(part); href != "" {
				out = append(out, collectionFileCandidate{Label: label, Source: href})
				continue
			}
			for _, text := range collectionPlainValuesFromText(label) {
				out = append(out, collectionFileCandidate{Label: text, Source: text})
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	values := collectionPlainValues(value)
	out := make([]collectionFileCandidate, 0, len(values))
	for _, item := range values {
		out = append(out, collectionFileCandidate{Label: fileLabel(item), Source: item})
	}
	return out
}

func richTextLinkDecoration(part []any) string {
	if len(part) < 2 {
		return ""
	}
	decorations, ok := part[1].([]any)
	if !ok {
		return ""
	}
	for _, deco := range decorations {
		item, ok := deco.([]any)
		if !ok || len(item) < 2 {
			continue
		}
		kind, _ := item[0].(string)
		if kind != "a" {
			continue
		}
		if href := strings.TrimSpace(stringValue(item[1])); href != "" {
			return href
		}
	}
	return ""
}

func fileLabel(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "attachment:") {
		label := strings.TrimSpace(strings.TrimPrefix(raw, "attachment:"))
		if decoded, err := url.PathUnescape(label); err == nil && decoded != "" {
			return decoded
		}
		if label != "" {
			return label
		}
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	label := path.Base(parsed.Path)
	if label == "." || label == "/" || label == "" {
		return raw
	}
	if decoded, err := url.PathUnescape(label); err == nil && decoded != "" {
		label = decoded
	}
	return label
}
