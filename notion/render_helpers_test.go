package notion

import (
	"encoding/json"
	"strings"
	"testing"
)

func marshalRecordMap(t *testing.T, blocks map[string]any) json.RawMessage {
	t.Helper()
	return marshalRecordMapObject(t, map[string]any{"block": blocks})
}

func groupedCollectionQuery(row1ID string, row2ID string, row3ID string) map[string]any {
	return map[string]any{
		"collection_group_results": map[string]any{
			"blockIds": []string{row1ID, row2ID, row3ID},
		},
		"results:status:Backlog":       map[string]any{"blockIds": []string{row1ID}},
		"results:status:Ready":         map[string]any{"blockIds": []string{row2ID}},
		"results:status:uncategorized": map[string]any{"blockIds": []string{row3ID}},
	}
}

func marshalRecordMapObject(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("rendered HTML missing %q: %s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("rendered HTML unexpectedly contains %q: %s", needle, haystack)
	}
}
