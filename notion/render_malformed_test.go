package notion

import (
	"testing"
)

// The renderer consumes untrusted Notion record-map JSON. Every case below is
// intentionally malformed (wrong types, empty rich-text arrays, junk
// decorations, dangling/cyclic content, hostile collection schemas) and must
// degrade gracefully: RenderPage may return an error or skip content, but it
// must never panic. These cases double as the seed corpus for
// FuzzRenderPageMalformed.
const malformedRootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

var malformedRecordMaps = map[string]string{
	"record map is an array":  `[]`,
	"record map is a string":  `"junk"`,
	"record map is a number":  `123`,
	"record map is null":      `null`,
	"block table is an array": `{"block": []}`,
	"block value is a number": `{"block": {"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": 5}}`,
	"block value is a string": `{"block": {"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": "junk"}}`,
	"root missing":            `{"block": {}}`,
	"empty rich text parts": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]},
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": {"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "type": "text", "properties": {"title": [[], [""], ["x"], [123], [null], [{}], ["y", null], ["z", "notdeco"]]}}
	}}`,
	"junk decorations": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]},
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": {"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "type": "text", "properties": {"title": [["text", [[], ["a"], ["a", 5], [123], [null, null], ["h", []], ["p"], ["u", {"x": 1}], ["e"], ["e", false]]]]}}
	}}`,
	"title is not an array": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "properties": {"title": {"nested": true}}, "content": ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]},
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": {"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "type": "text", "properties": {"title": 42}}
	}}`,
	"hostile page format": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "format": {"page_full_width": "yes", "page_small_text": 1, "page_font": 5, "block_color": [], "page_icon": {}, "page_cover": 9, "attributes": 7, "table_block_column_order": "junk", "table_block_column_format": []}}
	}}`,
	"dangling and cyclic content": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "missing-1", "cccccccc-cccc-cccc-cccc-cccccccccccc"]},
		"cccccccc-cccc-cccc-cccc-cccccccccccc": {"id": "cccccccc-cccc-cccc-cccc-cccccccccccc", "type": "column_list", "content": ["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "cccccccc-cccc-cccc-cccc-cccccccccccc"]}
	}}`,
	"hostile collection schema": `{
		"block": {
			"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["dddddddd-dddd-dddd-dddd-dddddddddddd"]},
			"dddddddd-dddd-dddd-dddd-dddddddddddd": {"id": "dddddddd-dddd-dddd-dddd-dddddddddddd", "type": "collection_view", "collection_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "view_ids": ["ffffffff-ffff-ffff-ffff-ffffffffffff", "missing-view"]}
		},
		"collection": {
			"eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee": {"id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee", "name": 42, "schema": {
				"aaa": {"name": 1, "type": [], "options": [null, "str", {"color": 5}, {"color": "red", "value": 3}, {"color": "blue"}], "formula": "notamap", "rollup": []},
				"bbb": {"formula": {"result_type": 7, "number_format": []}},
				"ccc": null
			}}
		},
		"collection_view": {
			"ffffffff-ffff-ffff-ffff-ffffffffffff": {"id": "ffffffff-ffff-ffff-ffff-ffffffffffff", "type": "table", "format": {"table_properties": "junk", "board_properties": [null, 5, {"property": 9, "visible": "no"}], "board_columns": {}}}
		},
		"collection_query": {
			"eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee": {"ffffffff-ffff-ffff-ffff-ffffffffffff": {"collection_group_results": {"blockIds": "junk"}}}
		}
	}`,
	"hostile timestamps and table rows": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "created_time": "junk", "last_edited_time": [], "content": ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]},
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": {"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "type": "table", "format": {"table_block_column_order": [1, null, {}], "table_block_row_header": "x"}, "content": ["gggggggg-gggg-gggg-gggg-gggggggggggg"]},
		"gggggggg-gggg-gggg-gggg-gggggggggggg": {"id": "gggggggg-gggg-gggg-gggg-gggggggggggg", "type": "table_row", "properties": {"col1": "notanarray", "col2": [[]]}}
	}}`,
	"junk block types and to_do": `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "cccccccc-cccc-cccc-cccc-cccccccccccc", "dddddddd-dddd-dddd-dddd-dddddddddddd"]},
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb": {"id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb", "type": "to_do", "properties": {"title": [["task"]], "checked": [["junk", 5]]}},
		"cccccccc-cccc-cccc-cccc-cccccccccccc": {"id": "cccccccc-cccc-cccc-cccc-cccccccccccc", "type": "unknown_block_type_zzz", "properties": {"title": [["?"]]}},
		"dddddddd-dddd-dddd-dddd-dddddddddddd": {"id": "dddddddd-dddd-dddd-dddd-dddddddddddd", "type": "equation", "properties": {"title": [["", [["e"]]], ["x", [["e", null]]]]}}
	}}`,
}

// Regression (found by FuzzRenderPageMalformed): a tiny record map with cyclic
// and repeated content references used to blow up exponentially — the 32-deep
// guard bounds depth but not fan-out, so ~350 bytes of JSON cost ~60s of CPU
// and unbounded output. The render budget must keep both runtime and output
// bounded; this test hanging or failing the size check means the budget
// regressed.
func TestRenderPageCyclicFanOutIsBounded(t *testing.T) {
	const raw = `{"block": {
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "content": ["", "", ""]},
		"": {"id": "cccccccc-cccc-cccc-cccc-cccccccccccc", "type": "column_list", "content": ["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ""]}
	}}`
	html, err := RenderPage(RenderInput{RecordMap: []byte(raw), PageID: malformedRootID})
	if err != nil {
		t.Fatal(err)
	}
	if len(html) > 1<<20 {
		t.Fatalf("render output not bounded: %d bytes", len(html))
	}
}

func TestRenderPageMalformedInputsDoNotPanic(t *testing.T) {
	for name, raw := range malformedRecordMaps {
		t.Run(name, func(t *testing.T) {
			// A panic here fails the test with the offending case name; the
			// renderer is allowed to return an error or empty/partial HTML.
			html, err := RenderPage(RenderInput{RecordMap: []byte(raw), PageID: malformedRootID})
			t.Logf("err=%v htmlLen=%d", err, len(html))
		})
	}
}

// FuzzRenderPageMalformed locks in panic-safety against arbitrary record-map
// bytes. `go test` runs only the seed corpus; run with
// `go test -fuzz=FuzzRenderPageMalformed ./notion` to explore further.
func FuzzRenderPageMalformed(f *testing.F) {
	for _, raw := range malformedRecordMaps {
		f.Add([]byte(raw))
	}
	f.Add([]byte(`{"block": {"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa": {"id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "type": "page", "properties": {"title": [["ok"]]}}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = RenderPage(RenderInput{RecordMap: data, PageID: malformedRootID})
	})
}
