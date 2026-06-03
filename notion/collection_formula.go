package notion

import (
	"encoding/json"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func renderCollectionComputedProperty(value any, schema collectionProperty, rm recordMap, input RenderInput) string {
	resultType := strings.ToLower(strings.TrimSpace(schema.ResultType))
	if rendered := renderCollectionStructuredComputedValue(value, resultType, schema, rm, input); rendered != "" {
		return rendered
	}
	switch resultType {
	case "number":
		return renderCollectionNumber(plainText(value), schema.NumberFormat)
	case "date":
		return renderCollectionDate(value)
	case "checkbox", "boolean":
		return renderCollectionCheckbox(plainText(value))
	case "person", "people", "created_by", "last_edited_by":
		return renderCollectionPeople(value)
	case "relation":
		return renderCollectionRelation(value, rm, input)
	case "file", "files":
		return renderCollectionFiles(value, rm, input)
	case "url":
		return renderCollectionURL(plainText(value))
	case "email":
		return renderEmailLink(plainText(value))
	case "phone_number":
		return renderPhoneLink(plainText(value))
	case "select", "multi_select", "status":
		return renderCollectionPills(collectionPlainValues(value), schema)
	case "string", "text", "title":
		return richTextWithResolver(value, notionMentionResolver(rm, input))
	}
	rendered := richTextWithResolver(value, notionMentionResolver(rm, input))
	if rendered == "" {
		return ""
	}
	text := plainText(value)
	if text != "" {
		if formatted := renderCollectionNumber(text, schema.NumberFormat); formatted != "" {
			return formatted
		}
	}
	return rendered
}

func renderCollectionStructuredComputedValue(value any, resultType string, schema collectionProperty, rm recordMap, input RenderInput) string {
	if items, ok := computedListItems(value); ok {
		return renderCollectionComputedList(items, resultType, schema, rm, input)
	}
	if payload, payloadType, ok := computedValuePayload(value); ok {
		if payloadType != "" {
			resultType = payloadType
		}
		if items, ok := computedListItems(payload); ok {
			return renderCollectionComputedList(items, resultType, schema, rm, input)
		}
		return renderCollectionComputedSingleValue(payload, resultType, schema, rm, input)
	}
	return ""
}

func renderCollectionComputedList(items []any, resultType string, schema collectionProperty, rm recordMap, input RenderInput) string {
	if len(items) == 0 {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(resultType)) {
	case "select", "multi_select", "status":
		labels := make([]string, 0, len(items))
		for _, item := range items {
			payload := item
			if unwrapped, _, ok := computedValuePayload(item); ok {
				payload = unwrapped
			}
			if text := computedPlainText(payload); text != "" {
				labels = append(labels, collectionPlainValuesFromText(text)...)
			}
		}
		return renderCollectionPills(labels, schema)
	}
	rendered := make([]string, 0, len(items))
	for _, item := range items {
		itemType := resultType
		payload := item
		if unwrapped, payloadType, ok := computedValuePayload(item); ok {
			payload = unwrapped
			if payloadType != "" {
				itemType = payloadType
			}
		}
		value := renderCollectionComputedSingleValue(payload, itemType, schema, rm, input)
		if value != "" {
			rendered = append(rendered, value)
		}
	}
	if len(rendered) == 0 {
		return ""
	}
	return `<span class="notion-property-rollup-list">` + strings.Join(rendered, `<span class="notion-property-rollup-separator">,</span>`) + `</span>`
}

func renderCollectionComputedSingleValue(value any, resultType string, schema collectionProperty, rm recordMap, input RenderInput) string {
	resultType = strings.ToLower(strings.TrimSpace(resultType))
	switch resultType {
	case "number":
		if number, ok := numberValue(value); ok {
			return `<span class="notion-property-number">` + html.EscapeString(formatCollectionNumber(number, schema.NumberFormat)) + `</span>`
		}
		return renderCollectionNumber(computedPlainText(value), schema.NumberFormat)
	case "date":
		return renderCollectionDate(value)
	case "checkbox", "boolean":
		if checked, ok := value.(bool); ok {
			return renderCollectionCheckbox(strconv.FormatBool(checked))
		}
		return renderCollectionCheckbox(computedPlainText(value))
	case "person", "people", "created_by", "last_edited_by":
		return renderCollectionPeople(value)
	case "relation":
		return renderCollectionRelation(value, rm, input)
	case "file", "files":
		return renderCollectionFiles(value, rm, input)
	case "url":
		return renderCollectionURL(computedPlainText(value))
	case "email":
		return renderEmailLink(computedPlainText(value))
	case "phone_number":
		return renderPhoneLink(computedPlainText(value))
	case "select", "multi_select", "status":
		return renderCollectionPills(collectionPlainValuesFromText(computedPlainText(value)), schema)
	case "string", "text", "title", "rich_text":
		if rich := richTextWithResolver(value, notionMentionResolver(rm, input)); rich != "" {
			return rich
		}
		return html.EscapeString(computedPlainText(value))
	default:
		if rich := richTextWithResolver(value, notionMentionResolver(rm, input)); rich != "" {
			return rich
		}
		text := computedPlainText(value)
		if text == "" {
			return ""
		}
		return html.EscapeString(text)
	}
}

func renderCollectionFormulaResult(value any, schema collectionProperty, rm recordMap, input RenderInput) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case bool:
		return renderCollectionCheckbox(strconv.FormatBool(typed))
	case float64:
		return `<span class="notion-property-number">` + html.EscapeString(formatCollectionNumber(typed, schema.NumberFormat)) + `</span>`
	case int:
		return `<span class="notion-property-number">` + html.EscapeString(formatCollectionNumber(float64(typed), schema.NumberFormat)) + `</span>`
	case time.Time:
		return renderCollectionDate(typed.Format(time.RFC3339))
	case string:
		switch strings.ToLower(strings.TrimSpace(schema.ResultType)) {
		case "number":
			return renderCollectionNumber(typed, schema.NumberFormat)
		case "checkbox", "boolean":
			return renderCollectionCheckbox(typed)
		case "date":
			return renderCollectionDate(typed)
		case "select", "multi_select", "status":
			return renderCollectionPills(collectionPlainValuesFromText(typed), schema)
		case "url":
			return renderCollectionURL(typed)
		case "email":
			return renderEmailLink(typed)
		case "phone_number":
			return renderPhoneLink(typed)
		default:
			return html.EscapeString(typed)
		}
	default:
		return renderCollectionComputedSingleValue(typed, schema.ResultType, schema, rm, input)
	}
}

// maxFormulaDepth bounds formula AST recursion. The AST comes from the
// (untrusted) record map, so a deeply nested formula must not be able to
// overflow the stack.
const maxFormulaDepth = 64

type formulaEvalContext struct {
	row   block
	coll  collection
	now   time.Time
	depth int
}

func evalCollectionFormula(formula map[string]any, row block, coll collection) (any, bool) {
	return evalCollectionFormulaNode(formula, formulaEvalContext{
		row:  row,
		coll: coll,
		now:  time.Now().UTC(),
	})
}

func evalCollectionFormulaNode(node any, ctx formulaEvalContext) (any, bool) {
	if ctx.depth > maxFormulaDepth {
		return nil, false
	}
	ctx.depth++
	data, ok := node.(map[string]any)
	if !ok {
		return node, node != nil
	}
	switch strings.ToLower(stringValue(data["type"])) {
	case "symbol":
		switch strings.ToLower(stringValue(data["name"])) {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return nil, false
		}
	case "constant":
		value := formulaRawString(data["value"])
		switch strings.ToLower(firstNonEmpty(stringValue(data["value_type"]), stringValue(data["result_type"]))) {
		case "number":
			number, err := strconv.ParseFloat(value, 64)
			return number, err == nil
		case "checkbox", "boolean":
			return formulaBool(value), true
		default:
			return value, true
		}
	case "property":
		return evalCollectionFormulaProperty(data, ctx)
	case "function", "operator":
		return evalCollectionFormulaFunction(data, ctx)
	default:
		return nil, false
	}
}

func formulaRawString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func evalCollectionFormulaProperty(data map[string]any, ctx formulaEvalContext) (any, bool) {
	propertyID := stringValue(data["id"])
	if propertyID == "" {
		return nil, false
	}
	value := ctx.row.Properties[propertyID]
	schema := ctx.coll.Schema[propertyID]
	resultType := strings.ToLower(firstNonEmpty(stringValue(data["result_type"]), schema.ResultType, schema.Type))
	if collectionPropertyValueEmpty(value) {
		switch schema.Type {
		case "created_time":
			value = blockTimestampValue(ctx.row.CreatedTime)
		case "last_edited_time":
			value = blockTimestampValue(ctx.row.LastEditedTime)
		default:
			return nil, resultType == "text" || resultType == "string"
		}
	}
	switch resultType {
	case "number":
		number, ok := formulaNumber(plainText(value))
		return number, ok
	case "checkbox", "boolean":
		return formulaBool(plainText(value)), true
	case "date":
		if date, ok := formulaDate(value, false); ok {
			return date, true
		}
		return nil, false
	default:
		return plainText(value), true
	}
}

func evalCollectionFormulaFunction(data map[string]any, ctx formulaEvalContext) (any, bool) {
	args, _ := anySlice(data["args"])
	arg := func(index int) (any, bool) {
		if index < 0 || index >= len(args) {
			return nil, false
		}
		return evalCollectionFormulaNode(args[index], ctx)
	}
	name := strings.TrimSpace(stringValue(data["name"]))
	switch name {
	case "if":
		condition, ok := arg(0)
		if !ok {
			return nil, false
		}
		if formulaTruthy(condition) {
			return arg(1)
		}
		return arg(2)
	case "and":
		left, okLeft := arg(0)
		right, okRight := arg(1)
		return formulaTruthy(left) && formulaTruthy(right), okLeft && okRight
	case "or":
		left, okLeft := arg(0)
		right, okRight := arg(1)
		return formulaTruthy(left) || formulaTruthy(right), okLeft && okRight
	case "not":
		value, ok := arg(0)
		return !formulaTruthy(value), ok
	case "equal":
		left, okLeft := arg(0)
		right, okRight := arg(1)
		return formulaString(left) == formulaString(right), okLeft && okRight
	case "unequal":
		left, okLeft := arg(0)
		right, okRight := arg(1)
		return formulaString(left) != formulaString(right), okLeft && okRight
	case "larger", "largerEq", "smaller", "smallerEq":
		leftRaw, okLeft := arg(0)
		rightRaw, okRight := arg(1)
		left, leftOK := formulaNumber(leftRaw)
		right, rightOK := formulaNumber(rightRaw)
		if !okLeft || !okRight || !leftOK || !rightOK {
			return nil, false
		}
		switch name {
		case "larger":
			return left > right, true
		case "largerEq":
			return left >= right, true
		case "smaller":
			return left < right, true
		default:
			return left <= right, true
		}
	case "add":
		left, okLeft := arg(0)
		right, okRight := arg(1)
		if !okLeft || !okRight {
			return nil, false
		}
		if leftNumber, ok := formulaNumber(left); ok {
			rightNumber, rightOK := formulaNumber(right)
			if !rightOK {
				return nil, false
			}
			return leftNumber + rightNumber, true
		}
		return formulaString(left) + formulaString(right), true
	case "concat":
		var b strings.Builder
		for i := range args {
			value, ok := arg(i)
			if !ok {
				return nil, false
			}
			b.WriteString(formulaString(value))
		}
		return b.String(), true
	case "length":
		value, ok := arg(0)
		return float64(len([]rune(formulaString(value)))), ok
	case "replace", "replaceAll":
		value, okValue := arg(0)
		pattern, okPattern := arg(1)
		replacement, okReplacement := arg(2)
		if !okValue || !okPattern || !okReplacement {
			return nil, false
		}
		re, err := regexp.Compile(formulaString(pattern))
		if err != nil {
			return nil, false
		}
		if name == "replace" {
			match := re.FindStringIndex(formulaString(value))
			if match == nil {
				return formulaString(value), true
			}
			text := formulaString(value)
			return text[:match[0]] + re.ReplaceAllString(text[match[0]:match[1]], formulaString(replacement)) + text[match[1]:], true
		}
		return re.ReplaceAllString(formulaString(value), formulaString(replacement)), true
	case "format":
		value, ok := arg(0)
		if !ok {
			return nil, false
		}
		if date, ok := value.(time.Time); ok {
			return date.Format("Jan 2, 2006"), true
		}
		return formulaString(value), true
	case "now":
		return ctx.now, true
	case "hour":
		value, ok := arg(0)
		date, dateOK := value.(time.Time)
		if !ok || !dateOK {
			return nil, false
		}
		return float64(date.Hour()), true
	case "minute":
		value, ok := arg(0)
		date, dateOK := value.(time.Time)
		if !ok || !dateOK {
			return nil, false
		}
		return float64(date.Minute()), true
	case "dateBetween":
		leftRaw, okLeft := arg(0)
		rightRaw, okRight := arg(1)
		unitRaw, okUnit := arg(2)
		left, leftOK := leftRaw.(time.Time)
		right, rightOK := rightRaw.(time.Time)
		if !okLeft || !okRight || !okUnit || !leftOK || !rightOK {
			return nil, false
		}
		return formulaDateBetween(left, right, formulaString(unitRaw)), true
	case "dateSubtract", "dateAdd":
		dateRaw, okDate := arg(0)
		numberRaw, okNumber := arg(1)
		unitRaw, okUnit := arg(2)
		date, dateOK := dateRaw.(time.Time)
		number, numberOK := formulaNumber(numberRaw)
		if !okDate || !okNumber || !okUnit || !dateOK || !numberOK {
			return nil, false
		}
		if name == "dateSubtract" {
			number = -number
		}
		return formulaDateAdd(date, number, formulaString(unitRaw)), true
	case "formatDate":
		dateRaw, okDate := arg(0)
		formatRaw, okFormat := arg(1)
		date, dateOK := dateRaw.(time.Time)
		if !okDate || !okFormat || !dateOK {
			return nil, false
		}
		return formulaFormatDate(date, formulaString(formatRaw)), true
	default:
		return nil, false
	}
}

func formulaNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case int:
		return float64(typed), true
	case bool:
		if typed {
			return 1, true
		}
		return 0, true
	case string:
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(typed), ",", ""), 64)
		return parsed, err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0)
	default:
		return numberValue(value)
	}
}

func formulaString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		return formatFloat(typed)
	case int:
		return strconv.Itoa(typed)
	case time.Time:
		return typed.Format("Jan 2, 2006 3:04 PM")
	default:
		return computedPlainText(typed)
	}
}

func formulaTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.TrimSpace(typed) != "" && strings.ToLower(strings.TrimSpace(typed)) != "false"
	case float64:
		return typed != 0 && !math.IsNaN(typed)
	case time.Time:
		return !typed.IsZero()
	default:
		return value != nil
	}
}

func formulaBool(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return normalized == "yes" || normalized == "true" || normalized == "checked" || normalized == "1"
}

func formulaDate(value any, end bool) (time.Time, bool) {
	if _, start, finish := richTextDateRangeMention(value); start != "" {
		dateValue := start
		if end && finish != "" {
			dateValue = finish
		}
		return parseFormulaDate(dateValue, "")
	}
	start, finish, startTime, endTime, ok := dateRangeFields(value)
	if !ok {
		if text := strings.TrimSpace(computedPlainText(value)); text != "" {
			return parseFormulaDate(text, "")
		}
		return time.Time{}, false
	}
	dateValue := start
	timeValue := startTime
	if end && finish != "" {
		dateValue = finish
		timeValue = endTime
	}
	return parseFormulaDate(dateValue, timeValue)
}

func parseFormulaDate(dateValue string, timeValue string) (time.Time, bool) {
	dateValue = strings.TrimSpace(dateValue)
	timeValue = strings.TrimSpace(timeValue)
	if dateValue == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339, dateValue); err == nil {
		return parsed.UTC(), true
	}
	if timeValue != "" {
		if parsed, err := time.ParseInLocation("2006-01-02 15:04", dateValue+" "+timeValue, time.UTC); err == nil {
			return parsed, true
		}
	}
	if parsed, err := time.ParseInLocation("2006-01-02", dateValue, time.UTC); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func formulaDateBetween(left time.Time, right time.Time, unit string) float64 {
	diff := left.Sub(right)
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "years", "year":
		return float64(left.Year() - right.Year())
	case "months", "month":
		return float64((left.Year()-right.Year())*12 + int(left.Month()) - int(right.Month()))
	case "hours", "hour":
		return math.Trunc(diff.Hours())
	case "minutes", "minute":
		return math.Trunc(diff.Minutes())
	default:
		return math.Trunc(diff.Hours() / 24)
	}
}

func formulaDateAdd(date time.Time, amount float64, unit string) time.Time {
	whole := int(math.Trunc(amount))
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "years", "year":
		return date.AddDate(whole, 0, 0)
	case "months", "month":
		return date.AddDate(0, whole, 0)
	case "hours", "hour":
		return date.Add(time.Duration(amount) * time.Hour)
	case "minutes", "minute":
		return date.Add(time.Duration(amount) * time.Minute)
	default:
		return date.AddDate(0, 0, whole)
	}
}

func formulaFormatDate(date time.Time, pattern string) string {
	switch strings.TrimSpace(pattern) {
	case "dddd":
		return date.Format("Monday")
	case "MMM d, yyyy":
		return date.Format("Jan 2, 2006")
	default:
		return date.Format("Jan 2, 2006")
	}
}

func computedListItems(value any) ([]any, bool) {
	if data, ok := value.(map[string]any); ok {
		for _, key := range []string{"array", "values", "results", "items"} {
			if items, ok := anySlice(data[key]); ok {
				return items, true
			}
		}
		if strings.EqualFold(stringValue(data["type"]), "array") {
			if items, ok := anySlice(data["value"]); ok {
				return items, true
			}
		}
	}
	items, ok := anySlice(value)
	if !ok || richTextArray(items) {
		return nil, false
	}
	return items, true
}

func computedValuePayload(value any) (any, string, bool) {
	data, ok := value.(map[string]any)
	if !ok {
		return nil, "", false
	}
	payloadType := strings.ToLower(strings.TrimSpace(stringValue(data["type"])))
	for _, key := range []string{"value", "result"} {
		if payload, exists := data[key]; exists {
			return normalizeComputedPayload(payload, payloadType)
		}
	}
	for _, key := range computedTypedPayloadKeys(payloadType) {
		if payload, exists := data[key]; exists {
			return normalizeComputedPayload(payload, payloadType)
		}
	}
	if payloadType == "date" {
		if _, ok := data["start_date"]; ok {
			return data, payloadType, true
		}
		if _, ok := data["start"]; ok {
			return data, payloadType, true
		}
	}
	return nil, "", false
}

func normalizeComputedPayload(payload any, payloadType string) (any, string, bool) {
	switch payloadType {
	case "formula", "rollup":
		if inner, innerType, ok := computedValuePayload(payload); ok {
			return inner, innerType, true
		}
	case "array":
		if items, ok := computedListItems(payload); ok {
			return items, payloadType, true
		}
	}
	return payload, payloadType, true
}

func computedTypedPayloadKeys(payloadType string) []string {
	switch payloadType {
	case "array":
		return []string{"array"}
	case "number":
		return []string{"number"}
	case "date":
		return []string{"date"}
	case "checkbox", "boolean":
		return []string{"boolean", "checkbox", "checked"}
	case "string", "text", "title":
		return []string{"string", "text", "title"}
	case "rich_text":
		return []string{"rich_text"}
	case "select", "multi_select", "status":
		return []string{"select", "multi_select", "status", "name"}
	case "person", "people", "created_by", "last_edited_by":
		return []string{"people", "person"}
	case "relation":
		return []string{"relation"}
	case "file", "files":
		return []string{"files", "file"}
	case "url":
		return []string{"url"}
	case "email":
		return []string{"email"}
	case "phone_number":
		return []string{"phone_number", "phone"}
	case "formula", "rollup":
		return []string{"formula", "rollup"}
	default:
		return nil
	}
}

func anySlice(value any) ([]any, bool) {
	items, ok := value.([]any)
	return items, ok
}

func richTextArray(items []any) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		part, ok := item.([]any)
		if !ok || len(part) == 0 {
			return false
		}
		if _, ok := part[0].(string); !ok {
			return false
		}
	}
	return true
}

func computedPlainText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return ""
		}
		return formatFloat(typed)
	case json.Number:
		return typed.String()
	case []any:
		if richTextArray(typed) {
			return plainText(typed)
		}
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := computedPlainText(item); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, ", ")
	case map[string]any:
		for _, key := range []string{"name", "title", "label", "plain_text", "text"} {
			if text := computedPlainText(typed[key]); text != "" {
				return text
			}
		}
		if date := firstNonEmpty(stringValue(typed["start_date"]), stringValue(typed["start"])); date != "" {
			return date
		}
		if payload, _, ok := computedValuePayload(typed); ok {
			return computedPlainText(payload)
		}
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(value))
	}
}
