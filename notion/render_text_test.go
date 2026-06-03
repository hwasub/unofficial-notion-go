package notion

import (
	"strings"
	"testing"
)

func TestRenderPageRendersTodoCheckedPropertyAndTableHeaders(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		todoID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		tableID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row1ID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row2ID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{todoID, tableID},
		},
		todoID: map[string]any{
			"id":   todoID,
			"type": "to_do",
			"properties": map[string]any{
				"title":   [][]any{{"Ship renderer"}},
				"checked": [][]any{{"Yes"}},
			},
		},
		tableID: map[string]any{
			"id":      tableID,
			"type":    "table",
			"content": []string{row1ID, row2ID},
			"format": map[string]any{
				"table_block_column_header": true,
				"table_block_row_header":    true,
				"table_block_column_order":  []any{"col1", "col2"},
			},
		},
		row1ID: map[string]any{
			"id":   row1ID,
			"type": "table_row",
			"properties": map[string]any{
				"col1": [][]any{{"Name"}},
				"col2": [][]any{{"Value"}},
			},
		},
		row2ID: map[string]any{
			"id":   row2ID,
			"type": "table_row",
			"properties": map[string]any{
				"col1": [][]any{{"Status"}},
				"col2": [][]any{{"Ready"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<p class="notion-todo notion-todo--checked"><input type="checkbox" disabled checked> <span class="notion-todo__text">Ship renderer</span></p>`)
	assertContains(t, html, `<table class="notion-simple-table"><colgroup><col><col></colgroup><tbody><tr><th scope="col">Name</th><th scope="col">Value</th></tr>`)
	assertContains(t, html, `<tr><th scope="row">Status</th><td>Ready</td></tr>`)
}
func TestRenderPagePreservesBlankBlocksAndCalloutColor(t *testing.T) {
	const (
		rootID    = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		blankID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		calloutID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{blankID, calloutID},
			"format": map[string]any{
				"page_icon": "📌",
			},
		},
		blankID: map[string]any{
			"id":   blankID,
			"type": "text",
		},
		calloutID: map[string]any{
			"id":   calloutID,
			"type": "callout",
			"properties": map[string]any{
				"title": [][]any{{"Remember"}},
			},
			"format": map[string]any{
				"page_icon":   "💡",
				"block_color": "teal_background",
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-page-icon-hero" aria-hidden="true">📌</div>`)
	assertContains(t, html, `<p class="notion-blank" aria-hidden="true">&nbsp;</p>`)
	assertContains(t, html, `<div class="notion-callout notion-color--teal-background">`)
	assertContains(t, html, `<div class="notion-callout__content">Remember</div>`)
}

func TestRenderPageDoesNotRenderNilTextAsLiteralNil(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		headerID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID, headerID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
		},
		headerID: map[string]any{
			"id":   headerID,
			"type": "header",
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap: recordMap,
		PageID:    rootID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(html, "&lt;nil&gt;") || strings.Contains(html, "<nil>") {
		t.Fatalf("nil rich text rendered as a literal nil marker: %s", html)
	}
	assertContains(t, html, `>Untitled</h2>`)
}

func TestRenderPageKeepsTextTodoAndQuoteChildrenAttached(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID       = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		textChildID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		todoID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		todoChildID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		quoteID      = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		quoteChildID = "11111111-1111-1111-1111-111111111111"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID, todoID, quoteID},
		},
		textID: map[string]any{
			"id":      textID,
			"type":    "text",
			"content": []string{textChildID},
			"properties": map[string]any{
				"title": [][]any{{"Parent text"}},
			},
		},
		textChildID: map[string]any{
			"id":   textChildID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Child text"}},
			},
		},
		todoID: map[string]any{
			"id":      todoID,
			"type":    "to_do",
			"content": []string{todoChildID},
			"properties": map[string]any{
				"title":   [][]any{{"Parent todo"}},
				"checked": [][]any{{"Yes"}},
			},
		},
		todoChildID: map[string]any{
			"id":   todoChildID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Todo detail"}},
			},
		},
		quoteID: map[string]any{
			"id":      quoteID,
			"type":    "quote",
			"content": []string{quoteChildID},
			"properties": map[string]any{
				"title": [][]any{{"Parent quote"}},
			},
		},
		quoteChildID: map[string]any{
			"id":   quoteChildID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Quote detail"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-block-with-children notion-block-with-children--text"><p>Parent text</p><div class="notion-block-children"><p>Child text</p></div></div>`)
	assertContains(t, html, `<div class="notion-block-with-children notion-block-with-children--todo"><p class="notion-todo notion-todo--checked"><input type="checkbox" disabled checked> <span class="notion-todo__text">Parent todo</span></p><div class="notion-block-children"><p>Todo detail</p></div></div>`)
	assertContains(t, html, `<blockquote><p>Parent quote</p><div class="notion-block-children"><p>Quote detail</p></div></blockquote>`)
	if strings.Contains(html, `</blockquote><p>Quote detail</p>`) || strings.Contains(html, `</div><p>Todo detail</p>`) {
		t.Fatalf("child blocks rendered as detached siblings: %s", html)
	}
}

func TestRenderPageGroupsConsecutiveLists(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		oneID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		twoID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		textID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		fourID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{oneID, twoID, textID, fourID},
		},
		oneID: map[string]any{
			"id":   oneID,
			"type": "numbered_list",
			"properties": map[string]any{
				"title": [][]any{{"First"}},
			},
		},
		twoID: map[string]any{
			"id":   twoID,
			"type": "numbered_list",
			"properties": map[string]any{
				"title": [][]any{{"Second"}},
			},
			"format": map[string]any{"list_start_index": 2},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Break"}},
			},
		},
		fourID: map[string]any{
			"id":   fourID,
			"type": "numbered_list",
			"properties": map[string]any{
				"title": [][]any{{"Fourth"}},
			},
			"format": map[string]any{"list_start_index": 4},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<ol><li>First</li><li>Second</li></ol>`)
	assertContains(t, html, `<ol start="4"><li>Fourth</li></ol>`)
	if strings.Contains(html, `<ol><li>First</li></ol><ol`) {
		t.Fatalf("consecutive numbered list items were rendered as separate lists: %s", html)
	}
}

func TestRenderPageRendersNativeTableColumnsInOrder(t *testing.T) {
	const (
		rootID  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tableID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{tableID},
		},
		tableID: map[string]any{
			"id":      tableID,
			"type":    "table",
			"content": []string{rowID},
			"format": map[string]any{
				"table_block_column_order": []string{"label", "value", "bad"},
				"table_block_column_format": map[string]any{
					"label": map[string]any{"color": "green_background", "width": 180},
					"value": map[string]any{"color": "red", "width": 90},
					"bad":   map[string]any{"color": `x" onmouseover="alert(1)`, "width": 9999},
				},
			},
		},
		rowID: map[string]any{
			"id":   rowID,
			"type": "table_row",
			"format": map[string]any{
				"block_color": "yellow_background",
			},
			"properties": map[string]any{
				"value": [][]any{{"20\nhours"}},
				"label": [][]any{{"Duration", [][]any{{"b"}}}},
				"bad":   [][]any{{"Ignored"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<table class="notion-simple-table"><colgroup><col style="width:180px"><col style="width:90px"><col></colgroup><tbody><tr class="notion-color--yellow-background"><td class="notion-color--green-background"><strong>Duration</strong></td><td class="notion-color--red">20<br>hours</td><td>Ignored</td></tr>`)
	if strings.Contains(html, `<tr></tr>`) {
		t.Fatalf("native table cells were dropped: %s", html)
	}
	if strings.Contains(html, "onmouseover") || strings.Contains(html, "9999px") {
		t.Fatalf("unsafe native table format leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageRendersToggleableHeadingsAsDetails(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		headerID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		childID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{headerID},
		},
		headerID: map[string]any{
			"id":      headerID,
			"type":    "header",
			"content": []string{childID},
			"properties": map[string]any{
				"title": [][]any{{"Toggle Header"}},
			},
			"format": map[string]any{
				"toggleable": true,
			},
		},
		childID: map[string]any{
			"id":   childID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Inside"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<details class="notion-toggle-heading notion-toggle-heading--h2" id="notion-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"><summary><h2 class="notion-toggle-heading__title">Toggle Header</h2></summary><p>Inside</p></details>`)
	if strings.Contains(html, `<h1`) {
		t.Fatalf("toggleable heading rendered as a flat heading: %s", html)
	}
}

func TestRenderPageShiftsContentHeadingsBelowPageTitle(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		h1ID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		h2ID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		h3ID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{h1ID, h2ID, h3ID},
		},
		h1ID: map[string]any{
			"id":   h1ID,
			"type": "header",
			"properties": map[string]any{
				"title": [][]any{{"Section"}},
			},
		},
		h2ID: map[string]any{
			"id":   h2ID,
			"type": "sub_header",
			"properties": map[string]any{
				"title": [][]any{{"Subsection"}},
			},
		},
		h3ID: map[string]any{
			"id":   h3ID,
			"type": "sub_sub_header",
			"properties": map[string]any{
				"title": [][]any{{"Detail"}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<h2 id="notion-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb">Section</h2>`)
	assertContains(t, html, `<h3 id="notion-cccccccccccccccccccccccccccccccc">Subsection</h3>`)
	assertContains(t, html, `<h4 id="notion-dddddddddddddddddddddddddddddddd">Detail</h4>`)
	if strings.Contains(html, `<h1 id="notion-`) {
		t.Fatalf("content heading should not compete with the page title h1: %s", html)
	}
}

func TestRenderPagePreservesColumnRatios(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		columnListID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		leftColumnID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		rightColID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		leftTextID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		rightTextID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{columnListID},
		},
		columnListID: map[string]any{
			"id":      columnListID,
			"type":    "column_list",
			"content": []string{leftColumnID, rightColID},
		},
		leftColumnID: map[string]any{
			"id":      leftColumnID,
			"type":    "column",
			"content": []string{leftTextID},
			"format": map[string]any{
				"column_ratio": 0.25,
			},
		},
		rightColID: map[string]any{
			"id":      rightColID,
			"type":    "column",
			"content": []string{rightTextID},
			"format": map[string]any{
				"column_ratio": 0.75,
			},
		},
		leftTextID: map[string]any{
			"id":   leftTextID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Left", nil}},
			},
		},
		rightTextID: map[string]any{
			"id":   rightTextID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Right", nil}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-column-list"><div class="notion-column" style="--notion-column-ratio:0.2500"><p>Left</p></div><div class="notion-column" style="--notion-column-ratio:0.7500"><p>Right</p></div></div>`)
}

func TestRenderPagePreservesAllowlistedBlockColors(t *testing.T) {
	const (
		rootID   = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		textID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		quoteID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		listID   = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		toggleID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{textID, quoteID, listID, toggleID},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"Text"}},
			},
			"format": map[string]any{
				"block_color": "red_background",
			},
		},
		quoteID: map[string]any{
			"id":   quoteID,
			"type": "quote",
			"properties": map[string]any{
				"title": [][]any{{"Quote"}},
			},
			"format": map[string]any{
				"block_color": "green",
			},
		},
		listID: map[string]any{
			"id":   listID,
			"type": "bulleted_list",
			"properties": map[string]any{
				"title": [][]any{{"List"}},
			},
			"format": map[string]any{
				"block_color": "purple_background",
			},
		},
		toggleID: map[string]any{
			"id":   toggleID,
			"type": "toggle",
			"properties": map[string]any{
				"title": [][]any{{"Toggle"}},
			},
			"format": map[string]any{
				"block_color": `x" onclick="alert(1)`,
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<p class="notion-color--red-background">Text</p>`)
	assertContains(t, html, `<blockquote class="notion-color--green">Quote</blockquote>`)
	assertContains(t, html, `<ul><li class="notion-color--purple-background">List</li></ul>`)
	assertContains(t, html, `<details class="notion-toggle"><summary>Toggle</summary></details>`)
	if strings.Contains(html, "onclick") {
		t.Fatalf("unsafe block color leaked into rendered HTML: %s", html)
	}
}

func TestRenderPageCodeBlockUsesLanguageClass(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		codeID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{codeID},
		},
		codeID: map[string]any{
			"id":   codeID,
			"type": "code",
			"properties": map[string]any{
				"title":   [][]any{{"func main() {\n\tprintln(\"hi\")\n}"}},
				"caption": [][]any{{"Caption <safe>", [][]any{{"b"}}}},
			},
			"format": map[string]any{
				"code_language": "Go",
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-code"><div class="notion-code__bar"><span class="notion-code__language">Go</span><button type="button" class="notion-code__copy" data-copy-code data-copy-success="Copied" data-copy-prompt="Copy code">Copy</button></div>`)
	// Code is emitted as a plain, escaped <pre><code> with a language-* class so
	// callers can apply CSS or a client-side highlighter; no server-side coloring.
	assertContains(t, html, `<pre><code class="language-go">func main() {`)
	assertContains(t, html, `println(&#34;hi&#34;)`)
	assertContains(t, html, `<div class="notion-code__caption"><strong>Caption &lt;safe&gt;</strong></div>`)
	if strings.Contains(html, "chroma") {
		t.Fatalf("code block must not carry server-side highlighter markup: %s", html)
	}
}

func TestRenderPageRendersEquationDecorationWithoutPlainTextDuplicate(t *testing.T) {
	const (
		rootID     = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		equationID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{equationID},
		},
		equationID: map[string]any{
			"id":   equationID,
			"type": "equation",
			"properties": map[string]any{
				"title": [][]any{{"plain rendered fallback", [][]any{{"e", `\frac{1}{2}`}}}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-equation" role="math" data-tex="\frac{1}{2}">\frac{1}{2}</div>`)
	if strings.Contains(html, "plain rendered fallback") {
		t.Fatalf("equation rendered Notion plaintext fallback: %s", html)
	}
}

func TestRenderPageStripsEquationPlainTextArtifacts(t *testing.T) {
	const (
		rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		eq1ID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		eq2ID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		eq3ID  = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		eq4ID  = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		textID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMap(t, map[string]any{
		rootID: map[string]any{
			"id":      rootID,
			"type":    "page",
			"content": []string{eq1ID, eq2ID, eq3ID, eq4ID, textID},
		},
		eq1ID: map[string]any{
			"id":   eq1ID,
			"type": "equation",
			"properties": map[string]any{
				"title": [][]any{{`\displaystyle \frac{1}{\Bigl(\sqrt{\phi \sqrt{5}}-\phi\Bigr) e^{\frac25 \pi}} = 1+\frac{e^{-2\pi}} {1+\frac{e^{-4\pi}} {1+\cdots} }(ϕ5−ϕ)e52π1=1+1+⋯`}},
			},
		},
		eq2ID: map[string]any{
			"id":   eq2ID,
			"type": "equation",
			"properties": map[string]any{
				"title": [][]any{{`\displaystyle \left( \sum_{k=1}^n a_k b_k \right)^2 \leq \left( \sum_{k=1}^n a_k^2 \right)(k=1∑nakbk)2≤...`}},
			},
		},
		eq3ID: map[string]any{
			"id":   eq3ID,
			"type": "equation",
			"properties": map[string]any{
				"title": [][]any{{`\displaystyle {1 + \frac{q^2}{(1-q)}+\cdots }= \prod_{j=0}^{\infty}\frac{1}{(1-q^{5j+2})(1-q^{5j+3})}, \quad \text{for }\lvert q\rvert<1.1+(1−q)q2+⋯`}},
			},
		},
		eq4ID: map[string]any{
			"id":   eq4ID,
			"type": "equation",
			"properties": map[string]any{
				"title": [][]any{{`[ x^n + y^n = z^n ]`}},
			},
		},
		textID: map[string]any{
			"id":   textID,
			"type": "text",
			"properties": map[string]any{
				"title": [][]any{{"inline", [][]any{{"e", `\(y = mx + b\)`}}}},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(html, "ϕ∑∏∣−⋯") || strings.Contains(html, `(k=1`) || strings.Contains(html, `1+(1`) {
		t.Fatalf("equation rendered Notion plaintext artifacts: %s", html)
	}
	if strings.Contains(html, `[ x^n`) || strings.Contains(html, `\(y =`) {
		t.Fatalf("equation rendered delimiter artifacts: %s", html)
	}
	assertContains(t, html, `<div class="notion-equation" role="math" data-tex="x^n + y^n = z^n">x^n + y^n = z^n</div>`)
	assertContains(t, html, `<span class="notion-equation notion-equation--inline" role="math" data-tex="y = mx + b">y = mx + b</span>`)
	assertContains(t, html, `\lvert q\rvert&lt;1.`)
}
