package notion

import (
	"strings"
	"testing"
	"time"
)

// TestFormatCollectionNumberPercentRounding guards against float artifacts in
// percent formatting (e.g. 0.07*100 -> 7.000000000000001).
func TestFormatCollectionNumberPercentRounding(t *testing.T) {
	cases := []struct {
		value  float64
		format string
		want   string
	}{
		{0.07, "percent", "7%"},
		{0.29, "percent", "29%"},
		{0.58, "percent", "58%"},
		{0.1034, "percent", "10.34%"},
		{-0.07, "percent", "-7%"},
		{0.07, "percent_with_commas", "7%"},
	}
	for _, c := range cases {
		if got := formatCollectionNumber(c.value, c.format); got != c.want {
			t.Errorf("formatCollectionNumber(%v, %q) = %q, want %q", c.value, c.format, got, c.want)
		}
	}
}

func TestEvalCollectionFormulaMathOperators(t *testing.T) {
	num := func(value string) map[string]any {
		return map[string]any{"type": "constant", "value": value, "value_type": "number"}
	}
	op := func(name string, args ...any) map[string]any {
		return map[string]any{"type": "operator", "name": name, "args": args}
	}
	cases := []struct {
		name string
		node map[string]any
		want float64
		ok   bool
	}{
		{"subtract", op("subtract", num("10"), num("4")), 6, true},
		{"multiply", op("multiply", num("6"), num("7")), 42, true},
		{"divide", op("divide", num("10"), num("4")), 2.5, true},
		{"divide by zero", op("divide", num("10"), num("0")), 0, false},
		// JS remainder semantics: the result takes the dividend's sign.
		{"mod", op("mod", num("-7"), num("3")), -1, true},
		{"mod by zero", op("mod", num("7"), num("0")), 0, false},
		{"pow", op("pow", num("2"), num("10")), 1024, true},
		{"pow overflow degrades", op("pow", num("10"), num("1000")), 0, false},
		{"unaryMinus", op("unaryMinus", num("5")), -5, true},
		{"unaryPlus", op("unaryPlus", num("5")), 5, true},
		{"toNumber", op("toNumber", num("3.5")), 3.5, true},
		{"abs", op("abs", num("-3.5")), 3.5, true},
		{"sqrt", op("sqrt", num("9")), 3, true},
		{"sqrt negative", op("sqrt", num("-1")), 0, false},
		{"floor", op("floor", num("2.9")), 2, true},
		{"ceil", op("ceil", num("2.1")), 3, true},
		// JS Math.round half-up, including for negative halves.
		{"round half up", op("round", num("2.5")), 3, true},
		{"round negative half", op("round", num("-2.5")), -2, true},
		{"min", op("min", num("4"), num("2"), num("9")), 2, true},
		{"max", op("max", num("4"), num("2"), num("9")), 9, true},
		{"nested", op("divide", op("add", num("1"), num("5")), num("3")), 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := evalCollectionFormula(tc.node, block{}, collection{})
			if ok != tc.ok {
				t.Fatalf("ok = %t, want %t (got %#v)", ok, tc.ok, got)
			}
			if !tc.ok {
				return
			}
			number, isNumber := got.(float64)
			if !isNumber || number != tc.want {
				t.Fatalf("result = %#v, want %v", got, tc.want)
			}
		})
	}
}

// Formula dates keep the offset their RFC3339 value carries; converting to UTC
// would shift the rendered wall-clock time (and hour()/minute()) away from
// what Notion shows for the same property.
func TestFormulaDatePreservesRFC3339Offset(t *testing.T) {
	date, ok := parseFormulaDate("2026-06-01T10:30:00+09:00", "")
	if !ok {
		t.Fatal("parseFormulaDate failed")
	}
	if got := date.Format(time.RFC3339); got != "2026-06-01T10:30:00+09:00" {
		t.Fatalf("formatted date = %q", got)
	}
	if got := date.Hour(); got != 10 {
		t.Fatalf("hour = %d, want wall-clock 10 in the value's own offset", got)
	}
}

func TestFormulaFormatDatePatterns(t *testing.T) {
	date := time.Date(2026, time.June, 3, 14, 5, 9, 0, time.UTC) // a Wednesday
	cases := []struct {
		pattern string
		want    string
	}{
		// The two patterns supported before the translator keep their output.
		{"dddd", "Wednesday"},
		{"MMM d, yyyy", "Jun 3, 2026"},
		{"YYYY-MM-DD", "2026-06-03"},
		{"yyyy-MM-dd", "2026-06-03"},
		{"MM/dd/yyyy", "06/03/2026"},
		{"dd/MM/yyyy", "03/06/2026"},
		{"MMMM D, YYYY", "June 3, 2026"},
		{"h:mm a", "2:05 pm"},
		{"hh:mm A", "02:05 PM"},
		{"H:mm", "14:05"},
		{"HH:mm:ss", "14:05:09"},
		{"ddd", "Wed"},
		{"[on] MMM d", "on Jun 3"},
		{"", "Jun 3, 2026"},
		{"!!!", "Jun 3, 2026"},
	}
	for _, tc := range cases {
		if got := formulaFormatDate(date, tc.pattern); got != tc.want {
			t.Errorf("formulaFormatDate(%q) = %q, want %q", tc.pattern, got, tc.want)
		}
	}
}

// TestCollectionGroupsHonorConfiguredOrder ensures grouped views emit groups in
// the user-configured order rather than alphabetically by query-result key.
func TestCollectionGroupsHonorConfiguredOrder(t *testing.T) {
	query := collectionQuery{
		Results: map[string]collectionResults{
			"results:status:Not started": {BlockIDs: []string{"row-a"}},
			"results:status:In progress": {BlockIDs: []string{"row-b"}},
			"results:status:Done":        {BlockIDs: []string{"row-c"}},
		},
	}
	specs := []collectionGroup{
		{Label: "Not started"},
		{Label: "In progress"},
		{Label: "Done"},
	}
	groups := collectionGroups(query, nil, specs, RenderInput{})
	got := make([]string, len(groups))
	for i, g := range groups {
		got[i] = g.Label
	}
	want := []string{"Not started", "In progress", "Done"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("group order = %v, want %v (must follow configured specs, not alphabetical)", got, want)
	}
}

// TestRenderCollectionPeopleComputedMap ensures a computed/rollup person value
// that arrives as a map renders the display name instead of raw Go map text.
// Specs whose labels do not survive the query-key round trip (e.g. "+" decodes
// to a space) or differ only in case must still land in their configured slot.
func TestCollectionGroupsMatchSpecsByConstructedKeyAndNormalizedLabel(t *testing.T) {
	query := collectionQuery{
		Results: map[string]collectionResults{
			"results:status:C++":   {BlockIDs: []string{"row-a"}},
			"results:status:ready": {BlockIDs: []string{"row-b"}},
		},
	}
	specs := []collectionGroup{
		{Label: "Ready", ValueType: "status", Collapsed: true},
		{Label: "C++", ValueType: "status", Hidden: true},
	}
	groups := collectionGroups(query, nil, specs, RenderInput{})
	if len(groups) != 2 {
		t.Fatalf("groups = %#v", groups)
	}
	if groups[0].Key != "results:status:ready" || groups[0].Label != "Ready" || !groups[0].Collapsed {
		t.Fatalf("normalized-label match failed: %#v", groups[0])
	}
	if groups[1].Key != "results:status:C++" || !groups[1].Hidden {
		t.Fatalf("constructed-key match failed: %#v", groups[1])
	}
}

// A later spec's exact-label match must win the bucket over an earlier spec's
// weaker normalized-label match.
func TestCollectionGroupsExactMatchBeatsEarlierNormalizedMatch(t *testing.T) {
	query := collectionQuery{
		Results: map[string]collectionResults{
			"results:status:ready": {BlockIDs: []string{"row-a"}},
		},
	}
	specs := []collectionGroup{
		{Label: "READY", ValueType: "status", Hidden: true},
		{Label: "ready", ValueType: "status", Collapsed: true},
	}
	groups := collectionGroups(query, nil, specs, RenderInput{})
	if len(groups) != 1 {
		t.Fatalf("groups = %#v", groups)
	}
	if groups[0].Label != "ready" || !groups[0].Collapsed || groups[0].Hidden {
		t.Fatalf("exact-match spec did not win the bucket: %#v", groups[0])
	}
}

func TestRenderCollectionUniqueID(t *testing.T) {
	schema := collectionProperty{Prefix: "TASK"}
	want := `<span class="notion-property-unique-id">TASK-12</span>`
	if got := renderCollectionUniqueID("12", schema); got != want {
		t.Fatalf("bare number = %q", got)
	}
	if got := renderCollectionUniqueID("TASK-12", schema); got != want {
		t.Fatalf("already prefixed = %q", got)
	}
	if got := renderCollectionUniqueID("7", collectionProperty{}); got != `<span class="notion-property-unique-id">7</span>` {
		t.Fatalf("no prefix = %q", got)
	}
	if got := renderCollectionUniqueID("  ", schema); got != "" {
		t.Fatalf("blank = %q", got)
	}
}

func TestRenderCollectionPeopleComputedMap(t *testing.T) {
	single := renderCollectionPeople(map[string]any{"name": "Alice"})
	if !strings.Contains(single, "Alice") || strings.Contains(single, "map[") {
		t.Fatalf("single person map = %q, want display name without raw map text", single)
	}
	if !strings.Contains(single, "notion-property-people") {
		t.Fatalf("single person map should render people chips: %q", single)
	}

	list := renderCollectionPeople([]any{
		map[string]any{"name": "Alice"},
		map[string]any{"name": "Bob"},
	})
	if !strings.Contains(list, "Alice") || !strings.Contains(list, "Bob") || strings.Contains(list, "map[") {
		t.Fatalf("person list = %q, want both names without raw map text", list)
	}

	// Rich-text arrays must keep their original rendering path.
	rich := renderCollectionPeople([]any{[]any{"Carol"}})
	if !strings.Contains(rich, "Carol") {
		t.Fatalf("rich-text person = %q, want name preserved", rich)
	}
}

// TestRenderCollectionURLTrims ensures the displayed link text is trimmed to
// match the normalized href.
func TestRenderCollectionURLTrims(t *testing.T) {
	got := renderCollectionURL("  https://example.com  ")
	if !strings.Contains(got, `>https://example.com</a>`) {
		t.Fatalf("renderCollectionURL = %q, want trimmed link text", got)
	}
	if strings.Contains(got, "> https") || strings.Contains(got, "com </a>") {
		t.Fatalf("renderCollectionURL = %q, link text should not contain surrounding whitespace", got)
	}
}
