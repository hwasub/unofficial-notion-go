package notion

import (
	"errors"
	"strings"
	"testing"
)

func validSnapshotForTest(t *testing.T) *Snapshot {
	t.Helper()
	const rootID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	return &Snapshot{
		SchemaVersion: SnapshotSchemaVersion,
		RootPageID:    rootID,
		Page: PageSnapshot{
			PageID: rootID,
			RecordMap: marshalRecordMap(t, map[string]any{
				rootID: map[string]any{"id": rootID, "type": "page"},
			}),
		},
	}
}

func TestValidateSnapshotAcceptsMatchingIdentity(t *testing.T) {
	snapshot := validSnapshotForTest(t)
	if err := ValidateSnapshot(snapshot, strings.ReplaceAll(snapshot.RootPageID, "-", "")); err != nil {
		t.Fatalf("ValidateSnapshot() error = %v", err)
	}
}

func TestValidateSnapshotRejectsInvalidSchemaAndIdentity(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Snapshot)
		want   error
	}{
		{"unsupported_schema", func(snapshot *Snapshot) { snapshot.SchemaVersion++ }, ErrUnsupportedSnapshotSchema},
		{"page_mismatch", func(snapshot *Snapshot) { snapshot.Page.PageID = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb" }, ErrInvalidSnapshot},
		{"missing_root_block", func(snapshot *Snapshot) { snapshot.Page.RecordMap = []byte(`{"block":{}}`) }, ErrInvalidSnapshot},
		{"root_block_mismatch", func(snapshot *Snapshot) {
			snapshot.Page.RecordMap = []byte(`{"block":{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa":{"id":"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb","type":"page"}}}`)
		}, ErrInvalidSnapshot},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := validSnapshotForTest(t)
			tt.change(snapshot)
			if err := ValidateSnapshot(snapshot, ""); !errors.Is(err, tt.want) {
				t.Fatalf("ValidateSnapshot() error = %v, want %v", err, tt.want)
			}
		})
	}

	if err := ValidateSnapshot(nil, ""); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("ValidateSnapshot(nil) error = %v, want ErrInvalidSnapshot", err)
	}
	snapshot := validSnapshotForTest(t)
	if err := ValidateSnapshot(snapshot, "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"); !errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("ValidateSnapshot(expected mismatch) error = %v, want ErrInvalidSnapshot", err)
	}
}

func TestCollectionsTruncatedProducesLocalizedRenderWarning(t *testing.T) {
	snapshot := validSnapshotForTest(t)
	snapshot.Truncated.Collections = true
	warnings := RenderWarningsForSnapshot(snapshot)
	if len(warnings) != 1 || warnings[0] != (RenderWarning{Kind: RenderWarningCollectionsTruncated, Count: 1}) {
		t.Fatalf("RenderWarningsForSnapshot() = %#v", warnings)
	}

	localized := "database rows are partial"
	html, err := RenderPage(RenderInput{
		RecordMap: snapshot.Page.RecordMap,
		PageID:    snapshot.RootPageID,
		Warnings:  warnings,
		T: func(key, fallback string) string {
			if key == "notion.warn_collections_truncated" {
				return localized
			}
			return fallback
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<li>"+localized+"</li>") {
		t.Fatalf("localized collection warning missing from %s", html)
	}
}
