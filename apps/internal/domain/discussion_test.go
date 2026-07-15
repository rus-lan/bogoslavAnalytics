package domain

import (
	"encoding/json"
	"testing"
)

// TestDiscussion_JSONRoundTrip_idTypesStayDistinct guards against the trap
// called out in TZ.md section 4.2: the discussion id is a string
// (SHA-like), the note id nested inside notes[] is an integer. They must
// never be conflated.
func TestDiscussion_JSONRoundTrip_idTypesStayDistinct(t *testing.T) {
	const raw = `{
		"id": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"individual_note": false,
		"notes": [
			{
				"id": 456,
				"type": "DiscussionNote",
				"body": "first reply",
				"author": {"id": 42, "username": "alice"},
				"created_at": "2026-03-01T10:00:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123
			},
			{
				"id": 457,
				"type": "DiscussionNote",
				"body": "second reply",
				"author": {"id": 43, "username": "bob"},
				"created_at": "2026-03-01T10:05:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123
			}
		]
	}`

	var d Discussion
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	wantID := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	if d.ID != wantID {
		t.Errorf("Discussion.ID = %q, want %q", d.ID, wantID)
	}
	if d.IndividualNote {
		t.Errorf("Discussion.IndividualNote = true, want false")
	}
	if len(d.Notes) != 2 {
		t.Fatalf("len(Notes) = %d, want 2", len(d.Notes))
	}
	if d.Notes[0].ID != 456 {
		t.Errorf("Notes[0].ID = %d, want 456", d.Notes[0].ID)
	}
	if d.Notes[1].ID != 457 {
		t.Errorf("Notes[1].ID = %d, want 457", d.Notes[1].ID)
	}

	out, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var roundTripped Discussion
	if err := json.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("Unmarshal(round-tripped) error = %v", err)
	}
	if roundTripped.ID != wantID {
		t.Errorf("round-tripped Discussion.ID = %q, want %q", roundTripped.ID, wantID)
	}
	if roundTripped.Notes[0].ID != 456 || roundTripped.Notes[1].ID != 457 {
		t.Errorf("round-tripped note ids = %d, %d, want 456, 457",
			roundTripped.Notes[0].ID, roundTripped.Notes[1].ID)
	}

	// Confirm the wire representation itself keeps the two ids as
	// distinct JSON types: the discussion id is a quoted string, the
	// note id is a bare number.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(out, &generic); err != nil {
		t.Fatalf("Unmarshal(generic) error = %v", err)
	}
	if string(generic["id"])[0] != '"' {
		t.Errorf("Discussion.id on the wire = %s, want a JSON string", generic["id"])
	}

	var genericNotes []map[string]json.RawMessage
	if err := json.Unmarshal(generic["notes"], &genericNotes); err != nil {
		t.Fatalf("Unmarshal(generic notes) error = %v", err)
	}
	if string(genericNotes[0]["id"])[0] == '"' {
		t.Errorf("Note.id on the wire = %s, want a bare JSON number, not a string", genericNotes[0]["id"])
	}
}
