package notion

import (
	"strings"
	"testing"
)

// Positive-path coverage for block renderers that were previously exercised
// only indirectly (or not at all): breadcrumb, tab, and inert button.

func TestRenderBreadcrumbBlock(t *testing.T) {
	const (
		parentID = "11111111-1111-1111-1111-111111111111"
		childID  = "22222222-2222-2222-2222-222222222222"
		crumbID  = "33333333-3333-3333-3333-333333333333"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		parentID: map[string]any{
			"id": parentID, "type": "page",
			"properties": map[string]any{"title": [][]any{{"Parent"}}},
		},
		childID: map[string]any{
			"id": childID, "type": "page", "parent_id": parentID,
			"properties": map[string]any{"title": [][]any{{"Child"}}},
			"content":    []string{crumbID},
		},
		crumbID: map[string]any{"id": crumbID, "type": "breadcrumb"},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: childID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `class="notion-breadcrumb"`)
	assertContains(t, html, `aria-current="page"`)
	assertContains(t, html, "Parent")
	assertContains(t, html, "Child")
}

func TestRenderTabBlock(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tabsID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		tab1ID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		tab2ID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{"id": rootID, "type": "page", "content": []string{tabsID}},
		tabsID: map[string]any{"id": tabsID, "type": "tab", "content": []string{tab1ID, tab2ID}},
		tab1ID: map[string]any{"id": tab1ID, "type": "tab", "properties": map[string]any{"title": [][]any{{"First"}}}},
		tab2ID: map[string]any{"id": tab2ID, "type": "tab", "properties": map[string]any{"title": [][]any{{"Second"}}}},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `role="tablist"`)
	assertContains(t, html, "First")
	assertContains(t, html, "Second")
	if got := strings.Count(html, `role="tab"`); got != 2 {
		t.Fatalf(`role="tab" count = %d, want 2: %s`, got, html)
	}
	if got := strings.Count(html, `aria-selected="true"`); got != 1 {
		t.Fatalf(`aria-selected="true" count = %d, want exactly 1 active tab`, got)
	}
}

func TestRenderInertButtonBlock(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		btnID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{"id": rootID, "type": "page", "content": []string{btnID}},
		btnID:  map[string]any{"id": btnID, "type": "button", "properties": map[string]any{"title": [][]any{{"Click me"}}}},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	// With no automation/action wired, the button must render inert and disabled
	// (it must never become a live actionable control in static HTML).
	assertContains(t, html, `class="notion-button-block"`)
	assertContains(t, html, "Click me")
	assertContains(t, html, "disabled")
	assertContains(t, html, `aria-disabled="true"`)
}
