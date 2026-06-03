package notion

import (
	"reflect"
	"testing"
)

// TestVisibleCollectionPropertySpecsFallbackIsDeterministic guards against the
// Table view reshuffling its columns between renders. When a collection view
// does not configure its visible properties, the renderer falls back to the
// title plus other schema properties; that fallback must pick a stable, sorted
// set rather than relying on Go's randomized map iteration order.
func TestVisibleCollectionPropertySpecsFallbackIsDeterministic(t *testing.T) {
	coll := collection{
		Schema: map[string]collectionProperty{
			"title": {Name: "Name", Type: "title"},
			"zzz":   {Name: "Zeta", Type: "text"},
			"aaa":   {Name: "Alpha", Type: "text"},
			"mmm":   {Name: "Mike", Type: "select"},
			"bbb":   {Name: "Bravo", Type: "number"},
			"kkk":   {Name: "Kilo", Type: "checkbox"},
			"ddd":   {Name: "Delta", Type: "date"},
			"ppp":   {Name: "Papa", Type: "url"},
		},
	}
	view := collectionView{Type: "table", Format: map[string]any{}}

	var first []string
	for i := 0; i < 25; i++ {
		specs := visibleCollectionPropertySpecs(view, coll)
		got := make([]string, len(specs))
		for j, s := range specs {
			got[j] = s.Property
		}
		if i == 0 {
			first = got
			continue
		}
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("non-deterministic fallback column order: run 0 = %v, run %d = %v", first, i, got)
		}
	}

	// title first, then the five lexicographically-smallest other properties
	// (capped at six columns total).
	want := []string{"title", "aaa", "bbb", "ddd", "kkk", "mmm"}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("unexpected fallback columns: got %v, want %v", first, want)
	}
}

// TestVisibleCollectionPropertySpecsEmptySchema ensures a missing or empty
// collection schema degrades to a single title column instead of panicking or
// emitting a column-less table.
func TestVisibleCollectionPropertySpecsEmptySchema(t *testing.T) {
	view := collectionView{Type: "table", Format: map[string]any{}}
	specs := visibleCollectionPropertySpecs(view, collection{})
	if len(specs) != 1 || specs[0].Property != "title" {
		t.Fatalf("empty schema should yield a single title column, got %+v", specs)
	}
}
