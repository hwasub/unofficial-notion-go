package notion

import (
	"strings"
	"testing"
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
