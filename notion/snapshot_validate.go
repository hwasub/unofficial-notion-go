package notion

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// SnapshotSchemaVersion is the snapshot wire format version supported by this
// release of the renderer.
const SnapshotSchemaVersion = 1

var (
	// ErrInvalidSnapshot identifies a malformed snapshot or one whose root,
	// page, and record-map identities disagree.
	ErrInvalidSnapshot = errors.New("notion: invalid snapshot")
	// ErrUnsupportedSnapshotSchema identifies a snapshot wire format this
	// release does not understand.
	ErrUnsupportedSnapshotSchema = errors.New("notion: unsupported snapshot schema")
)

// ValidateSnapshot strictly validates a transported Snapshot before it is
// stored or rendered. expectedPageID may be empty; when provided, it must be a
// valid Notion page ID matching both RootPageID and Page.PageID. The record map
// must also contain a root block whose own ID matches those fields.
//
// Validation does not mutate or scrub the snapshot. Call
// ScrubSnapshotForStorage separately before persistence.
func ValidateSnapshot(snapshot *Snapshot, expectedPageID string) error {
	if snapshot == nil {
		return fmt.Errorf("%w: snapshot is nil", ErrInvalidSnapshot)
	}
	if snapshot.SchemaVersion != SnapshotSchemaVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedSnapshotSchema, snapshot.SchemaVersion, SnapshotSchemaVersion)
	}
	rootID, ok := normalizedPageID(snapshot.RootPageID)
	if !ok {
		return fmt.Errorf("%w: invalid root_page_id %q", ErrInvalidSnapshot, snapshot.RootPageID)
	}
	pageID, ok := normalizedPageID(snapshot.Page.PageID)
	if !ok {
		return fmt.Errorf("%w: invalid page.page_id %q", ErrInvalidSnapshot, snapshot.Page.PageID)
	}
	if pageID != rootID {
		return fmt.Errorf("%w: root_page_id %q does not match page.page_id %q", ErrInvalidSnapshot, snapshot.RootPageID, snapshot.Page.PageID)
	}
	if strings.TrimSpace(expectedPageID) != "" {
		expectedID, valid := normalizedPageID(expectedPageID)
		if !valid {
			return fmt.Errorf("%w: invalid expected page ID %q", ErrInvalidSnapshot, expectedPageID)
		}
		if expectedID != rootID {
			return fmt.Errorf("%w: expected page ID %q does not match root_page_id %q", ErrInvalidSnapshot, expectedPageID, snapshot.RootPageID)
		}
	}

	var recordMap struct {
		Block map[string]json.RawMessage `json:"block"`
	}
	if len(snapshot.Page.RecordMap) == 0 {
		return fmt.Errorf("%w: record_map is empty", ErrInvalidSnapshot)
	}
	if err := json.Unmarshal(snapshot.Page.RecordMap, &recordMap); err != nil {
		return fmt.Errorf("%w: invalid record_map JSON: %v", ErrInvalidSnapshot, err)
	}
	var rootBlockJSON json.RawMessage
	for id, raw := range recordMap.Block {
		if normalized, valid := normalizedPageID(id); valid && normalized == rootID {
			rootBlockJSON = raw
			break
		}
	}
	if len(rootBlockJSON) == 0 {
		return fmt.Errorf("%w: root block %q is missing from record_map", ErrInvalidSnapshot, rootID)
	}
	var rootBlock struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rootBlockJSON, &rootBlock); err != nil {
		return fmt.Errorf("%w: invalid root block: %v", ErrInvalidSnapshot, err)
	}
	blockID, valid := normalizedPageID(rootBlock.ID)
	if !valid || blockID != rootID {
		return fmt.Errorf("%w: root block ID %q does not match root_page_id %q", ErrInvalidSnapshot, rootBlock.ID, snapshot.RootPageID)
	}
	return nil
}
