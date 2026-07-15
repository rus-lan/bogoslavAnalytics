package classify

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// sampleNote builds a minimal, valid domain.Note fixture for a batch
// entry, with the given id and body.
func sampleNote(id int64, body string) domain.Note {
	return domain.Note{
		ID:           id,
		Body:         body,
		Author:       domain.Author{ID: 42, Username: "alice"},
		CreatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
		System:       false,
		NoteableID:   77,
		NoteableType: "MergeRequest",
		ProjectID:    123,
	}
}

// assertEqualJSON compares got and want by re-marshaling both to JSON
// rather than with reflect.DeepEqual, since time.Time (carried inside
// domain.Note) is documented as unsafe to compare directly — the same
// convention internal/domain and internal/artifact use for their own
// tests.
func assertEqualJSON[T any](t *testing.T, got, want T) {
	t.Helper()

	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal got: %v", err)
	}
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("mismatch:\n got = %s\nwant = %s", gotJSON, wantJSON)
	}
}
