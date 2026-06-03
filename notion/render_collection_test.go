package notion

import (
	"strings"
	"testing"
)

func TestRenderPageRendersCollectionViewRows(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		viewID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		relatedID    = "11111111-1111-1111-1111-111111111111"
		notionSpec   = "https://file.notion.so/workspace/local-spec.pdf"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Launch"}},
					"due":    [][]any{{"‣", [][]any{{"d", map[string]any{"start_date": "2026-06-01"}}}}},
					"status": [][]any{{"Ready"}},
					"tags":   [][]any{{"alpha,beta"}},
					"done":   [][]any{{"Yes"}},
					"owner":  [][]any{{"‣", [][]any{{"u", "user-id"}}}},
					"url":    [][]any{{"https://example.com"}},
					"budget": [][]any{{"12345.5"}},
					"phone":  [][]any{{"+1 (555) 010-9999"}},
					"spec":   [][]any{{"https://cdn.example.com/files/spec.pdf"}},
					"local":  [][]any{{"local-spec.pdf", [][]any{{"a", notionSpec}}}},
					"score":  [][]any{{"0.875"}},
					"rel":    [][]any{{"‣", [][]any{{"p", relatedID}}}},
				},
				"format": map[string]any{
					"page_icon": "🧪",
				},
			},
			relatedID: map[string]any{
				"id":   relatedID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Related page"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Roadmap"}},
				"schema": map[string]any{
					"title": map[string]any{"name": "Name", "type": "title"},
					"due":   map[string]any{"name": "Due", "type": "date"},
					"status": map[string]any{"name": "Status", "type": "status", "options": []map[string]any{
						{"value": "Ready", "color": "green"},
					}},
					"tags": map[string]any{"name": "Tags", "type": "multi_select", "options": []map[string]any{
						{"value": "alpha", "color": "blue"},
						{"value": "beta", "color": "red_background"},
						{"value": "ignored", "color": `red" onclick="alert(1)`},
					}},
					"done":   map[string]any{"name": "Done", "type": "checkbox"},
					"owner":  map[string]any{"name": "Owner", "type": "person"},
					"url":    map[string]any{"name": "URL", "type": "url"},
					"budget": map[string]any{"name": "Budget", "type": "number", "number_format": "dollar_with_commas"},
					"phone":  map[string]any{"name": "Phone", "type": "phone_number"},
					"spec":   map[string]any{"name": "Spec", "type": "files"},
					"local":  map[string]any{"name": "Local", "type": "files"},
					"score":  map[string]any{"name": "Score", "type": "formula", "number_format": "percent"},
					"rel":    map[string]any{"name": "Related", "type": "relation"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"name": "Default",
				"format": map[string]any{
					"table_wrap": true,
					"table_properties": []map[string]any{
						{"property": "title", "visible": true, "width": 222},
						{"property": "due", "visible": true, "width": 151},
						{"property": "status", "visible": true},
						{"property": "tags", "visible": true},
						{"property": "done", "visible": true},
						{"property": "owner", "visible": true},
						{"property": "url", "visible": true},
						{"property": "budget", "visible": true},
						{"property": "phone", "visible": true},
						{"property": "spec", "visible": true},
						{"property": "local", "visible": true},
						{"property": "score", "visible": true},
						{"property": "rel", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{
						"blockIds": []string{rowID},
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "roadmap",
		PagePaths:    map[string]string{rowID: rowID, relatedID: relatedID},
		AssetURLs:    map[string]string{notionSpec: "/notion-assets/local/local-spec.pdf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--table">`)
	assertContains(t, html, `<strong>Roadmap</strong>`)
	assertContains(t, html, `<table class="notion-collection-table notion-collection-table--wrap"><colgroup><col style="width:222px"><col style="width:151px">`)
	assertContains(t, html, `<span class="notion-property-head__icon notion-property-head__icon--title" aria-hidden="true"></span><span class="notion-property-head__label">Name</span>`)
	assertContains(t, html, `<span class="notion-property-head__icon notion-property-head__icon--date" aria-hidden="true"></span><span class="notion-property-head__label">Due</span>`)
	assertContains(t, html, `<span class="notion-property-head__icon notion-property-head__icon--number" aria-hidden="true"></span><span class="notion-property-head__label">Budget</span>`)
	assertContains(t, html, `<span class="notion-property-head__icon notion-property-head__icon--formula" aria-hidden="true"></span><span class="notion-property-head__label">Score</span>`)
	assertContains(t, html, `<td><a class="notion-collection-title" href="/roadmap/cccccccc-cccc-cccc-cccc-cccccccccccc"><span class="notion-page-icon" aria-hidden="true">🧪</span>Launch</a></td>`)
	assertContains(t, html, `<td><span class="notion-property-pills"><span class="notion-property-pill notion-color--green-background">Ready</span></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-pills"><span class="notion-property-pill notion-color--blue-background">alpha</span><span class="notion-property-pill notion-color--red-background">beta</span></span></td>`)
	if strings.Contains(html, "onclick") {
		t.Fatalf("unsafe option color leaked into rendered HTML: %s", html)
	}
	assertContains(t, html, `<td><span class="notion-property-checkbox"><input type="checkbox" disabled aria-label="Checked" checked></span></td>`)
	assertContains(t, html, `<td><span class="notion-mention">@Person</span></td>`)
	assertContains(t, html, `<td><a href="https://example.com" rel="noopener noreferrer">https://example.com</a></td>`)
	assertContains(t, html, `<td><span class="notion-property-number">$12,345.50</span></td>`)
	assertContains(t, html, `<td><a href="tel:+15550109999">+1 (555) 010-9999</a></td>`)
	assertContains(t, html, `<td><span class="notion-property-files"><a href="https://cdn.example.com/files/spec.pdf" rel="noopener noreferrer">spec.pdf</a></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-files"><a href="/notion-assets/local/local-spec.pdf" rel="noopener noreferrer">local-spec.pdf</a></span></td>`)
	if strings.Contains(html, `href="`+notionSpec+`"`) {
		t.Fatalf("rendered Notion-hosted collection file URL directly: %s", html)
	}
	assertContains(t, html, `<td><span class="notion-property-number">87.5%</span></td>`)
	assertContains(t, html, `<td><a class="notion-mention notion-mention--page" href="/roadmap/11111111-1111-1111-1111-111111111111">Related page</a></td>`)
}

func TestRenderPageRendersCollectionAggregations(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		viewID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Launch"}},
					"budget": [][]any{{"12345.5"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Roadmap"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"budget": map[string]any{"name": "Budget", "type": "number", "number_format": "dollar_with_commas"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"format": map[string]any{
					"table_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "budget", "visible": true},
					},
				},
				"query2": map[string]any{
					"aggregations": []map[string]any{
						{"property": "title", "aggregator": "count"},
						{"property": "budget", "aggregator": "sum"},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{
						"blockIds": []string{rowID},
						"aggregationResults": []map[string]any{
							{"type": "number", "value": 1},
							{"type": "number", "value": 12345.5},
						},
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-collection-aggregations"><span><strong>Name Count</strong> 1</span><span><strong>Budget Sum</strong> $12,345.50</span></div>`)
}

func TestRenderPageDropsUnsafeCollectionAggregationLabels(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		viewID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Launch"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Roadmap"}},
				"schema": map[string]any{
					"title": map[string]any{"name": "Name", "type": "title"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"query2": map[string]any{
					"aggregations": []map[string]any{
						{"property": "title", "aggregator": `sum" onclick="alert(1)`},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{
						"blockIds": []string{rowID},
						"aggregationResults": []map[string]any{
							{"type": "number", "value": 1},
						},
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-collection-aggregations"><span><strong>Name</strong> 1</span></div>`)
	if strings.Contains(html, "onclick") || strings.Contains(html, "alert") || strings.Contains(html, "sum&quot;") {
		t.Fatalf("unsafe aggregation label leaked into rendered HTML: %s", html)
	}
}

func TestRenderCollectionFilesUsesRichTextAssetLinks(t *testing.T) {
	const (
		cachedSource = "https://file.notion.so/workspace/cached-spec.pdf?table=block&id=abc"
		staleSource  = "https://file.notion.so/workspace/stale-spec.pdf?table=block&id=def"
	)
	value := []any{
		[]any{"Cached <Spec>.pdf", []any{[]any{"a", cachedSource}}},
		[]any{","},
		[]any{"Stale <Spec>.pdf", []any{[]any{"a", staleSource}}},
		[]any{","},
		[]any{"External.pdf", []any{[]any{"a", "https://example.com/files/external.pdf"}}},
	}

	html := renderCollectionFiles(value, recordMap{}, RenderInput{AssetURLs: map[string]string{
		"https://file.notion.so/workspace/cached-spec.pdf": "/notion-assets/local/cached-spec.pdf",
	}})

	assertContains(t, html, `<a href="/notion-assets/local/cached-spec.pdf" rel="noopener noreferrer">Cached &lt;Spec&gt;.pdf</a>`)
	assertContains(t, html, `<span>Stale &lt;Spec&gt;.pdf</span>`)
	assertContains(t, html, `<a href="https://example.com/files/external.pdf" rel="noopener noreferrer">External.pdf</a>`)
	if strings.Contains(html, "file.notion.so") {
		t.Fatalf("rendered a Notion-hosted collection file URL directly: %s", html)
	}
}

func TestRenderPageRendersComputedCollectionPropertyTypes(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		viewID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":               rowID,
				"type":             "page",
				"created_time":     float64(1780297200000),
				"last_edited_time": "2026-06-02T09:30:00.000Z",
				"properties": map[string]any{
					"title":    [][]any{{"Computed item"}},
					"price":    [][]any{{"1234.5"}},
					"progress": [][]any{{"0.421"}},
					"ok":       [][]any{{"true"}},
					"when": [][]any{{"‣", [][]any{{"d", map[string]any{
						"type":       "date",
						"start_date": "2026-06-03",
					}}}}},
					"label": [][]any{{"123"}},
					"roll_tags": map[string]any{
						"type": "array",
						"value": []any{
							map[string]any{"type": "select", "value": "Alpha"},
							map[string]any{"name": "Beta <unsafe>"},
						},
					},
					"roll_numbers": map[string]any{
						"type": "array",
						"value": []any{
							map[string]any{"type": "number", "value": 12.5},
							float64(30),
						},
					},
					"roll_dates": map[string]any{
						"type": "array",
						"value": []any{
							map[string]any{"type": "date", "value": map[string]any{"start_date": "2026-06-04", "end_date": "2026-06-04", "end_time": "14:30"}},
							map[string]any{"start_date": "2026-06-05"},
						},
					},
					"roll_wrapped": map[string]any{
						"type": "rollup",
						"rollup": map[string]any{
							"type": "array",
							"array": []any{
								map[string]any{"type": "formula", "formula": map[string]any{"type": "number", "number": 12.5}},
								map[string]any{"type": "formula", "formula": map[string]any{"type": "string", "string": `Nested <formula>`}},
							},
						},
					},
					"roll_rich_text": map[string]any{
						"type": "array",
						"value": []any{
							map[string]any{"type": "rich_text", "rich_text": [][]any{{"Rich <safe>", [][]any{{"b"}}}}},
						},
					},
					"typed_amount": map[string]any{"type": "number", "number": 9876.5},
					"typed_due":    map[string]any{"type": "date", "date": map[string]any{"start": "2026-06-06", "end": "2026-06-08"}},
					"typed_text":   map[string]any{"type": "string", "string": `Unsafe <formula>`},
					"typed_ok":     map[string]any{"type": "boolean", "boolean": false},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Computed"}},
				"schema": map[string]any{
					"title":        map[string]any{"name": "Name", "type": "title"},
					"price":        map[string]any{"name": "Price", "type": "number", "number_format": "canadian_dollar"},
					"progress":     map[string]any{"name": "Progress", "type": "formula", "formula": map[string]any{"type": "number", "number_format": "percent_with_commas"}},
					"ok":           map[string]any{"name": "OK", "type": "formula", "formula": map[string]any{"type": "checkbox"}},
					"when":         map[string]any{"name": "When", "type": "rollup", "rollup": map[string]any{"type": "date"}},
					"label":        map[string]any{"name": "Label", "type": "formula", "formula": map[string]any{"type": "string"}},
					"roll_tags":    map[string]any{"name": "Roll Tags", "type": "rollup", "rollup": map[string]any{"type": "multi_select"}},
					"roll_numbers": map[string]any{"name": "Roll Numbers", "type": "rollup", "rollup": map[string]any{"type": "number", "number_format": "dollar_with_commas"}},
					"roll_dates":   map[string]any{"name": "Roll Dates", "type": "rollup", "rollup": map[string]any{"type": "date"}},
					"roll_wrapped": map[string]any{"name": "Roll Wrapped", "type": "rollup", "rollup": map[string]any{"type": "array", "number_format": "dollar_with_commas"}},
					"roll_rich_text": map[string]any{
						"name":   "Roll Rich Text",
						"type":   "rollup",
						"rollup": map[string]any{"type": "array"},
					},
					"typed_amount": map[string]any{"name": "Typed Amount", "type": "formula", "formula": map[string]any{"type": "number", "number_format": "euro"}},
					"typed_due":    map[string]any{"name": "Typed Due", "type": "formula", "formula": map[string]any{"type": "date"}},
					"typed_text":   map[string]any{"name": "Typed Text", "type": "formula", "formula": map[string]any{"type": "string"}},
					"typed_ok":     map[string]any{"name": "Typed OK", "type": "formula", "formula": map[string]any{"type": "checkbox"}},
					"created":      map[string]any{"name": "Created", "type": "created_time"},
					"edited":       map[string]any{"name": "Edited", "type": "last_edited_time"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"name": "Computed table",
				"format": map[string]any{
					"table_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "price", "visible": true},
						{"property": "progress", "visible": true},
						{"property": "ok", "visible": true},
						{"property": "when", "visible": true},
						{"property": "label", "visible": true},
						{"property": "roll_tags", "visible": true},
						{"property": "roll_numbers", "visible": true},
						{"property": "roll_dates", "visible": true},
						{"property": "roll_wrapped", "visible": true},
						{"property": "roll_rich_text", "visible": true},
						{"property": "typed_amount", "visible": true},
						{"property": "typed_due", "visible": true},
						{"property": "typed_text", "visible": true},
						{"property": "typed_ok", "visible": true},
						{"property": "created", "visible": true},
						{"property": "edited", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{rowID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<td><span class="notion-property-number">C$1,234.50</span></td>`)
	assertContains(t, html, `<td><span class="notion-property-number">42.1%</span></td>`)
	assertContains(t, html, `<td><span class="notion-property-checkbox"><input type="checkbox" disabled aria-label="Checked" checked></span></td>`)
	assertContains(t, html, `<td><span class="notion-date-mention"><time datetime="2026-06-03">Jun 3, 2026</time></span></td>`)
	assertContains(t, html, `<td>123</td>`)
	assertContains(t, html, `<td><span class="notion-property-pills"><span class="notion-property-pill">Alpha</span><span class="notion-property-pill">Beta &lt;unsafe&gt;</span></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-rollup-list"><span class="notion-property-number">$12.50</span><span class="notion-property-rollup-separator">,</span><span class="notion-property-number">$30.00</span></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-rollup-list"><span class="notion-date-mention"><time datetime="2026-06-04">Jun 4, 2026 → 2:30 PM</time></span><span class="notion-property-rollup-separator">,</span><span class="notion-date-mention"><time datetime="2026-06-05">Jun 5, 2026</time></span></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-rollup-list"><span class="notion-property-number">$12.50</span><span class="notion-property-rollup-separator">,</span>Nested &lt;formula&gt;</span></td>`)
	assertContains(t, html, `<td><span class="notion-property-rollup-list"><strong>Rich &lt;safe&gt;</strong></span></td>`)
	assertContains(t, html, `<td><span class="notion-property-number">€9,876.50</span></td>`)
	assertContains(t, html, `<td><span class="notion-date-mention"><time datetime="2026-06-06">Jun 6, 2026 → Jun 8, 2026</time></span></td>`)
	assertContains(t, html, `<td>Unsafe &lt;formula&gt;</td>`)
	assertContains(t, html, `<td><span class="notion-property-checkbox"><input type="checkbox" disabled aria-label="Unchecked"></span></td>`)
	assertContains(t, html, `<td><span class="notion-date-mention"><time datetime="2026-06-01T07:00:00Z">Jun 1, 2026 7:00 AM</time></span></td>`)
	assertContains(t, html, `<td><span class="notion-date-mention"><time datetime="2026-06-02T09:30:00Z">Jun 2, 2026 9:30 AM</time></span></td>`)
}

func TestRenderCollectionEvaluatesFormulaSchemaWhenValueMissing(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		collectionID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		viewID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		rowID        = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":           rowID,
				"type":         "page",
				"parent_id":    collectionID,
				"parent_table": "collection",
				"properties": map[string]any{
					"title": [][]any{{"Formula row"}},
					"date":  [][]any{{"‣", [][]any{{"d", map[string]any{"type": "date", "start_date": "2026-06-01"}}}}},
					"tags":  [][]any{{"alpha,beta,gamma"}},
					"ok":    [][]any{{"No"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Formula Eval"}},
				"schema": map[string]any{
					"title": map[string]any{"name": "Name", "type": "title"},
					"date":  map[string]any{"name": "Date", "type": "date"},
					"tags":  map[string]any{"name": "Tags", "type": "multi_select"},
					"ok":    map[string]any{"name": "OK", "type": "checkbox"},
					"date_text": map[string]any{
						"name": "Date Text",
						"type": "formula",
						"formula": map[string]any{
							"type":        "operator",
							"name":        "add",
							"result_type": "text",
							"args": []any{
								map[string]any{"type": "constant", "value": "Test: ", "value_type": "string", "result_type": "text"},
								map[string]any{"type": "function", "name": "format", "result_type": "text", "args": []any{
									map[string]any{"type": "property", "id": "date", "name": "Date", "result_type": "date"},
								}},
							},
						},
					},
					"tag_count": map[string]any{
						"name": "Tag Count",
						"type": "formula",
						"formula": map[string]any{
							"type":        "function",
							"name":        "if",
							"result_type": "number",
							"args": []any{
								map[string]any{"type": "operator", "name": "larger", "result_type": "checkbox", "args": []any{
									map[string]any{"type": "function", "name": "length", "result_type": "number", "args": []any{
										map[string]any{"type": "property", "id": "tags", "name": "Tags", "result_type": "text"},
									}},
									map[string]any{"type": "constant", "value": "0", "value_type": "number", "result_type": "number"},
								}},
								map[string]any{"type": "operator", "name": "add", "result_type": "number", "args": []any{
									map[string]any{"type": "function", "name": "length", "result_type": "number", "args": []any{
										map[string]any{"type": "function", "name": "replaceAll", "result_type": "text", "args": []any{
											map[string]any{"type": "property", "id": "tags", "name": "Tags", "result_type": "text"},
											map[string]any{"type": "constant", "value": "[^,]", "value_type": "string", "result_type": "text"},
											map[string]any{"type": "constant", "value": "", "value_type": "string", "result_type": "text"},
										}},
									}},
									map[string]any{"type": "constant", "value": "1", "value_type": "number", "result_type": "number"},
								}},
								map[string]any{"type": "constant", "value": "0", "value_type": "number", "result_type": "number"},
							},
						},
					},
					"bool_formula": map[string]any{
						"name": "Bool",
						"type": "formula",
						"formula": map[string]any{
							"type":        "function",
							"name":        "if",
							"result_type": "checkbox",
							"args": []any{
								map[string]any{"type": "operator", "name": "not", "result_type": "checkbox", "args": []any{
									map[string]any{"type": "function", "name": "and", "result_type": "checkbox", "args": []any{
										map[string]any{"type": "property", "id": "ok", "name": "OK", "result_type": "checkbox"},
										map[string]any{"type": "symbol", "name": "true", "result_type": "checkbox"},
									}},
								}},
								map[string]any{"type": "symbol", "name": "true", "result_type": "checkbox"},
								map[string]any{"type": "symbol", "name": "false", "result_type": "checkbox"},
							},
						},
					},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"format": map[string]any{
					"table_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "date_text", "visible": true},
						{"property": "tag_count", "visible": true},
						{"property": "bool_formula", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{rowID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<td>Test: Jun 1, 2026</td>`)
	assertContains(t, html, `<td><span class="notion-property-number">3</span></td>`)
	assertContains(t, html, `<td><span class="notion-property-checkbox"><input type="checkbox" disabled aria-label="Checked" checked></span></td>`)
}

func TestFormatCollectionNumberMatchesNotionCurrencyPrecision(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		format string
		want   string
	}{
		{name: "dollar cents", value: 35002, format: "dollar", want: "$35,002.00"},
		{name: "euro negative cents", value: -20.32, format: "euro", want: "-€20.32"},
		{name: "yen whole", value: 35002, format: "yen", want: "¥35,002"},
		{name: "won whole negative", value: -20.32, format: "won", want: "-₩20"},
		{name: "yuan cents", value: 1, format: "yuan", want: "CN¥1.00"},
		{name: "percent", value: 10.34, format: "percent_with_commas", want: "1,034%"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatCollectionNumber(tc.value, tc.format); got != tc.want {
				t.Fatalf("formatCollectionNumber(%v, %q) = %q, want %q", tc.value, tc.format, got, tc.want)
			}
		})
	}
}

func TestRenderCollectionDatePreservesRFC3339Offset(t *testing.T) {
	html := renderCollectionDate(map[string]any{
		"start": "2026-06-01T10:30:00+09:00",
		"end":   "2026-06-01T12:00:00+09:00",
	})
	assertContains(t, html, `<span class="notion-date-mention"><time datetime="2026-06-01T10:30:00+09:00">Jun 1, 2026 10:30 AM → 12:00 PM</time></span>`)
	if strings.Contains(html, "01:30") || strings.Contains(html, "03:00") {
		t.Fatalf("date range was converted to UTC instead of preserving the explicit offset: %s", html)
	}
}

func TestRenderPageRendersCollectionListAndGalleryViews(t *testing.T) {
	const (
		rootID         = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		listBlockID    = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		galleryBlockID = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		rowID          = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		collectionID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		listViewID     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		galleryViewID  = "11111111-1111-1111-1111-111111111111"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{listBlockID, galleryBlockID},
			},
			listBlockID: map[string]any{
				"id":            listBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{listViewID},
			},
			galleryBlockID: map[string]any{
				"id":            galleryBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{galleryViewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Gallery item"}},
					"status": [][]any{{"Ready"}},
				},
				"format": map[string]any{
					"page_icon": "🎨",
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Views"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "select"},
				},
			},
		},
		"collection_view": map[string]any{
			listViewID: map[string]any{
				"id":   listViewID,
				"type": "list",
				"name": "List",
				"format": map[string]any{
					"list_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
			galleryViewID: map[string]any{
				"id":   galleryViewID,
				"type": "gallery",
				"name": "Gallery",
				"format": map[string]any{
					"gallery_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				listViewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{rowID}},
				},
				galleryViewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{rowID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{
		RecordMap:    recordMap,
		PageID:       rootID,
		ResourceSlug: "views",
		PagePaths:    map[string]string{rowID: rowID},
		AssetURLs:    map[string]string{rowID: "/notion-assets/page-cover/cover.jpg"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--list">`)
	assertContains(t, html, `<div class="notion-collection-list">`)
	assertContains(t, html, `<article class="notion-collection-list__item"><strong><a class="notion-collection-title" href="/views/dddddddd-dddd-dddd-dddd-dddddddddddd">`)
	assertContains(t, html, `<section class="notion-collection notion-collection--gallery">`)
	assertContains(t, html, `<article class="notion-collection-card notion-collection-card--gallery"><div class="notion-collection-card__cover"><a class="notion-collection-card__cover-link" href="/notion-assets/page-cover/cover.jpg" data-collection-lightbox data-full-src="/notion-assets/page-cover/cover.jpg"><img src="/notion-assets/page-cover/cover.jpg"`)
}

func TestRenderPageRendersMultipleCollectionViewsAsTabs(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		listViewID   = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		boardViewID  = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{listViewID, boardViewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Tabbed row"}},
					"status": [][]any{{"Ready"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Tabbed database"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "status"},
				},
			},
		},
		"collection_view": map[string]any{
			listViewID: map[string]any{
				"id":   listViewID,
				"type": "list",
				"name": "List",
				"format": map[string]any{
					"list_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
			boardViewID: map[string]any{
				"id":   boardViewID,
				"type": "board",
				"name": `Board <unsafe>`,
				"format": map[string]any{
					"board_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				listViewID: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{rowID}},
				},
				boardViewID: map[string]any{
					"board_columns": map[string]any{
						"results": []map[string]any{
							{"value": map[string]any{"type": "status", "value": "Ready"}},
						},
					},
					"results:status:Ready": map[string]any{"blockIds": []string{rowID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--views">`)
	assertContains(t, html, `<div class="notion-tab-block notion-collection-view-tabs" data-notion-tabs>`)
	assertContains(t, html, `role="tab" id="notion-collection-view-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-tab-0" aria-selected="true"`)
	assertContains(t, html, `data-notion-tab-target="notion-collection-view-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-panel-1"><span class="notion-tab-button__icon notion-tab-button__icon--board" aria-hidden="true"></span><span class="notion-tab-button__label">Board &lt;unsafe&gt;</span></button>`)
	assertContains(t, html, `<section class="notion-tab-panel notion-collection-view-panel notion-collection-view-panel--list" role="tabpanel" tabindex="0" id="notion-collection-view-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-panel-0"`)
	assertContains(t, html, `<section class="notion-tab-panel notion-collection-view-panel notion-collection-view-panel--board" role="tabpanel" tabindex="0" id="notion-collection-view-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb-panel-1"`)
	assertContains(t, html, `data-notion-tab-panel hidden><div class="notion-collection-board">`)
	assertContains(t, html, `<article class="notion-collection-list__item"><strong>Tabbed row</strong>`)
	assertContains(t, html, `<section class="notion-collection-board__lane"><h3>Ready</h3>`)
	if strings.Contains(html, `<unsafe>`) {
		t.Fatalf("collection view tab label was not escaped: %s", html)
	}
}

func TestRenderPageRendersGroupedCollectionViews(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		tableBlockID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		listBlockID  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row1ID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row2ID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		row3ID       = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		collectionID = "11111111-1111-1111-1111-111111111111"
		tableViewID  = "22222222-2222-2222-2222-222222222222"
		listViewID   = "33333333-3333-3333-3333-333333333333"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{tableBlockID, listBlockID},
			},
			tableBlockID: map[string]any{
				"id":            tableBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{tableViewID},
			},
			listBlockID: map[string]any{
				"id":            listBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{listViewID},
			},
			row1ID: map[string]any{
				"id":   row1ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Backlog item"}},
					"status": [][]any{{"Backlog"}},
				},
			},
			row2ID: map[string]any{
				"id":   row2ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Ready item"}},
					"status": [][]any{{"Ready"}},
				},
			},
			row3ID: map[string]any{
				"id":   row3ID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Unsorted item"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Grouped views"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "status"},
				},
			},
		},
		"collection_view": map[string]any{
			tableViewID: map[string]any{
				"id":   tableViewID,
				"type": "table",
				"name": "Grouped table",
				"format": map[string]any{
					"table_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
			listViewID: map[string]any{
				"id":   listViewID,
				"type": "list",
				"name": "Grouped list",
				"format": map[string]any{
					"list_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				tableViewID: groupedCollectionQuery(row1ID, row2ID, row3ID),
				listViewID:  groupedCollectionQuery(row1ID, row2ID, row3ID),
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--table">`)
	assertContains(t, html, `<div class="notion-collection-groups"><section class="notion-collection-group"><h3><span class="notion-collection-group__label">Backlog</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<section class="notion-collection-group"><h3><span class="notion-collection-group__label">Ready</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<section class="notion-collection-group"><h3><span class="notion-collection-group__label">No value</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<td>Backlog item</td>`)
	assertContains(t, html, `<td>Ready item</td>`)
	assertContains(t, html, `<td>Unsorted item</td>`)
	assertContains(t, html, `<section class="notion-collection notion-collection--list">`)
	assertContains(t, html, `<article class="notion-collection-list__item"><strong>Backlog item</strong>`)
}

func TestRenderPageDerivesGroupedCollectionViewPageFromFormat(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		row1ID       = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row2ID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row3ID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		collectionID = "11111111-1111-1111-1111-111111111111"
		viewID       = "22222222-2222-2222-2222-222222222222"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view_page",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			row1ID: map[string]any{
				"id":   row1ID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"First row"}},
					"tags":  [][]any{{"group 2"}},
				},
			},
			row2ID: map[string]any{
				"id":   row2ID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Second row"}},
				},
			},
			row3ID: map[string]any{
				"id":   row3ID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Third row"}},
					"tags":  [][]any{{"group 1, group 2"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Grouped from format"}},
				"schema": map[string]any{
					"title": map[string]any{"name": "Name", "type": "title"},
					"tags":  map[string]any{"name": "Tags", "type": "multi_select"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "table",
				"name": "Default view",
				"format": map[string]any{
					"table_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "tags", "visible": true},
					},
					"collection_group_by": map[string]any{
						"property": "tags",
						"type":     "multi_select",
					},
					"collection_groups": []map[string]any{
						{"property": "tags", "value": map[string]any{"type": "multi_select"}},
						{"property": "tags", "value": map[string]any{"type": "multi_select", "value": "group 1"}},
						{"property": "tags", "value": map[string]any{"type": "multi_select", "value": "group 2"}},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"collection_group_results": map[string]any{
						"blockIds": []string{row1ID, row2ID, row3ID},
					},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--table notion-collection--view-page">`)
	assertContains(t, html, `<div class="notion-collection-groups"><section class="notion-collection-group"><h3><span class="notion-collection-group__label">No value</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<section class="notion-collection-group"><h3><span class="notion-collection-group__label">group 1</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<section class="notion-collection-group"><h3><span class="notion-collection-group__label">group 2</span><span class="notion-collection-group__count">2</span></h3>`)
	assertContains(t, html, `<td>First row</td>`)
	assertContains(t, html, `<td>Second row</td>`)
	assertContains(t, html, `<td>Third row</td>`)
}

func TestRenderPageHonorsCollectionGroupVisibilityAndCollapse(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		row1ID       = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row2ID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row3ID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		collectionID = "11111111-1111-1111-1111-111111111111"
		viewID       = "22222222-2222-2222-2222-222222222222"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view_page",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			row1ID: map[string]any{
				"id":   row1ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Collapsed row"}},
					"status": [][]any{{"Backlog"}},
				},
			},
			row2ID: map[string]any{
				"id":   row2ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Ready row"}},
					"status": [][]any{{"Ready"}},
				},
			},
			row3ID: map[string]any{
				"id":   row3ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Archived row"}},
					"status": [][]any{{"Archived"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Grouped visibility"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "status"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "list",
				"name": "Visible groups",
				"format": map[string]any{
					"collection_group_by": map[string]any{
						"property": "status",
						"type":     "status",
					},
					"collection_groups": []map[string]any{
						{"property": "status", "value": map[string]any{"type": "status", "value": "Backlog"}, "is_collapsed": true},
						{"property": "status", "value": map[string]any{"type": "status", "value": "Ready"}, "visible": true},
						{"property": "status", "value": map[string]any{"type": "status", "value": "Archived"}, "visible": false},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"results:status:Backlog":  map[string]any{"blockIds": []string{row1ID}},
					"results:status:Ready":    map[string]any{"blockIds": []string{row2ID}},
					"results:status:Archived": map[string]any{"blockIds": []string{row3ID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<details class="notion-collection-group notion-collection-group--collapsed"><summary><span class="notion-collection-group__label">Backlog</span><span class="notion-collection-group__count">1</span></summary>`)
	assertContains(t, html, `<section class="notion-collection-group"><h3><span class="notion-collection-group__label">Ready</span><span class="notion-collection-group__count">1</span></h3>`)
	assertContains(t, html, `<strong>Collapsed row</strong>`)
	assertContains(t, html, `<strong>Ready row</strong>`)
	assertNotContains(t, html, `Archived row`)
	assertNotContains(t, html, `>Archived<`)
}

func TestRenderPageDoesNotFallBackToUngroupedRowsWhenAllCollectionGroupsAreHidden(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rowID        = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		collectionID = "11111111-1111-1111-1111-111111111111"
		viewID       = "22222222-2222-2222-2222-222222222222"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view_page",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			rowID: map[string]any{
				"id":   rowID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Hidden row"}},
					"status": [][]any{{"Archived"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Hidden groups"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "status"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "list",
				"name": "Hidden groups",
				"format": map[string]any{
					"collection_group_by": map[string]any{
						"property": "status",
						"type":     "status",
					},
					"collection_groups": []map[string]any{
						{"property": "status", "value": map[string]any{"type": "status", "value": "Archived"}, "hidden": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"results:status:Archived": map[string]any{"blockIds": []string{rowID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<div class="notion-collection-groups"></div>`)
	assertNotContains(t, html, `Hidden row`)
	assertNotContains(t, html, `Archived`)
}

func TestRenderPageRendersBoardCollectionLanes(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		viewBlockID  = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		row1ID       = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row2ID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row3ID       = "11111111-1111-1111-1111-111111111111"
		collectionID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		viewID       = "ffffffff-ffff-ffff-ffff-ffffffffffff"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{viewBlockID},
			},
			viewBlockID: map[string]any{
				"id":            viewBlockID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{viewID},
			},
			row1ID: map[string]any{
				"id":   row1ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Backlog card"}},
					"status": [][]any{{"Backlog"}},
				},
			},
			row2ID: map[string]any{
				"id":   row2ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Ready card"}},
					"status": [][]any{{"Ready"}},
				},
			},
			row3ID: map[string]any{
				"id":   row3ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Archived card"}},
					"status": [][]any{{"Archived"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Board"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"status": map[string]any{"name": "Status", "type": "select"},
				},
			},
		},
		"collection_view": map[string]any{
			viewID: map[string]any{
				"id":   viewID,
				"type": "board",
				"name": "Board view",
				"format": map[string]any{
					"board_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				viewID: map[string]any{
					"board_columns": map[string]any{
						"results": []map[string]any{
							{"value": map[string]any{"type": "select"}, "visible": true},
							{"value": map[string]any{"type": "select", "value": "Ready"}, "visible": true},
							{"value": map[string]any{"type": "select", "value": "Archived"}, "visible": false},
						},
					},
					"results:select:uncategorized": map[string]any{"blockIds": []string{row1ID}},
					"results:select:Ready":         map[string]any{"blockIds": []string{row2ID}},
					"results:select:Archived":      map[string]any{"blockIds": []string{row3ID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--board">`)
	assertContains(t, html, `<div class="notion-collection-board">`)
	assertContains(t, html, `<section class="notion-collection-board__lane"><h3>No status</h3>`)
	assertContains(t, html, `<strong>Backlog card</strong>`)
	assertContains(t, html, `<section class="notion-collection-board__lane"><h3>Ready</h3>`)
	assertContains(t, html, `<strong>Ready card</strong>`)
	if strings.Contains(html, "Archived") || strings.Contains(html, `<h3>Other</h3>`) {
		t.Fatalf("hidden board column leaked into rendered board: %s", html)
	}
}

func TestRenderPageRendersCalendarAndTimelineCollectionViews(t *testing.T) {
	const (
		rootID       = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		calendarID   = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		timelineID   = "cccccccc-cccc-cccc-cccc-cccccccccccc"
		row1ID       = "dddddddd-dddd-dddd-dddd-dddddddddddd"
		row2ID       = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"
		collectionID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		calendarView = "11111111-1111-1111-1111-111111111111"
		timelineView = "22222222-2222-2222-2222-222222222222"
	)
	recordMap := marshalRecordMapObject(t, map[string]any{
		"block": map[string]any{
			rootID: map[string]any{
				"id":      rootID,
				"type":    "page",
				"content": []string{calendarID, timelineID},
			},
			calendarID: map[string]any{
				"id":            calendarID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{calendarView},
			},
			timelineID: map[string]any{
				"id":            timelineID,
				"type":          "collection_view",
				"collection_id": collectionID,
				"view_ids":      []string{timelineView},
			},
			row1ID: map[string]any{
				"id":   row1ID,
				"type": "page",
				"properties": map[string]any{
					"title": [][]any{{"Scheduled item"}},
					"due": [][]any{{"‣", [][]any{{"d", map[string]any{
						"type":       "date",
						"start_date": "2026-06-01",
						"end_date":   "2026-06-02",
					}}}}},
					"status": [][]any{{"Ready"}},
				},
			},
			row2ID: map[string]any{
				"id":   row2ID,
				"type": "page",
				"properties": map[string]any{
					"title":  [][]any{{"Unscheduled item"}},
					"status": [][]any{{"Backlog"}},
				},
			},
		},
		"collection": map[string]any{
			collectionID: map[string]any{
				"id":   collectionID,
				"name": [][]any{{"Schedule"}},
				"schema": map[string]any{
					"title":  map[string]any{"name": "Name", "type": "title"},
					"due":    map[string]any{"name": "Due", "type": "date"},
					"status": map[string]any{"name": "Status", "type": "status"},
				},
			},
		},
		"collection_view": map[string]any{
			calendarView: map[string]any{
				"id":   calendarView,
				"type": "calendar",
				"name": "Calendar",
				"format": map[string]any{
					"calendar_by": "due",
					"calendar_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "due", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
			timelineView: map[string]any{
				"id":   timelineView,
				"type": "timeline",
				"name": "Timeline",
				"format": map[string]any{
					"timeline_by": map[string]any{"property": "due"},
					"timeline_properties": []map[string]any{
						{"property": "title", "visible": true},
						{"property": "due", "visible": true},
						{"property": "status", "visible": true},
					},
				},
			},
		},
		"collection_query": map[string]any{
			collectionID: map[string]any{
				calendarView: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{row1ID, row2ID}},
				},
				timelineView: map[string]any{
					"collection_group_results": map[string]any{"blockIds": []string{row1ID, row2ID}},
				},
			},
		},
	})

	html, err := RenderPage(RenderInput{RecordMap: recordMap, PageID: rootID})
	if err != nil {
		t.Fatal(err)
	}
	assertContains(t, html, `<section class="notion-collection notion-collection--calendar">`)
	assertContains(t, html, `<div class="notion-collection-calendar">`)
	assertContains(t, html, `<section class="notion-collection-calendar__day"><h3><time datetime="2026-06-01">Jun 1, 2026 → Jun 2, 2026</time></h3>`)
	assertContains(t, html, `<section class="notion-collection-calendar__day notion-collection-calendar__day--unscheduled"><h3>No date</h3>`)
	assertContains(t, html, `<section class="notion-collection notion-collection--timeline">`)
	assertContains(t, html, `<div class="notion-collection-timeline__axis" style="--notion-timeline-days:2" aria-hidden="true"><span>Jun 1</span><span>Jun 2</span></div>`)
	assertContains(t, html, `<article class="notion-collection-timeline__item" style="--notion-timeline-days:2;--notion-timeline-start:1;--notion-timeline-span:2"><div class="notion-collection-timeline__date"><time datetime="2026-06-01">Jun 1, 2026 → Jun 2, 2026</time></div><div class="notion-collection-timeline__bar">`)
	assertContains(t, html, `<section class="notion-collection-timeline__unscheduled"><h3>No date</h3>`)
	assertContains(t, html, `<strong>Scheduled item</strong>`)
	assertContains(t, html, `<strong>Unscheduled item</strong>`)
}
