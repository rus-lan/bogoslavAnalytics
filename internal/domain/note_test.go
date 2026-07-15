package domain

import (
	"encoding/json"
	"testing"
)

func TestNoteType_MarshalJSON(t *testing.T) {
	cases := []struct {
		name string
		in   NoteType
		want string
	}{
		{"none becomes null", NoteTypeNone, "null"},
		{"discussion note", NoteTypeDiscussion, `"DiscussionNote"`},
		{"diff note", NoteTypeDiff, `"DiffNote"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Marshal() = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestNoteType_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  NoteType
	}{
		{"null becomes none", "null", NoteTypeNone},
		{"discussion note", `"DiscussionNote"`, NoteTypeDiscussion},
		{"diff note", `"DiffNote"`, NoteTypeDiff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got NoteType
			if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if got != tc.want {
				t.Errorf("Unmarshal() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestNote_JSONRoundTrip_typeField exercises the field the whole
// DiscussionNote trap hinges on: a plain, non-threaded comment carries
// JSON null for "type", while thread replies carry a string. See TZ.md
// section 4.2.
func TestNote_JSONRoundTrip_typeField(t *testing.T) {
	cases := []struct {
		name     string
		rawType  string
		wantType NoteType
	}{
		{"plain comment has null type", `null`, NoteTypeNone},
		{"thread reply has DiscussionNote type", `"DiscussionNote"`, NoteTypeDiscussion},
		{"diff comment has DiffNote type", `"DiffNote"`, NoteTypeDiff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{
				"id": 456,
				"type": ` + tc.rawType + `,
				"body": "looks good to me",
				"author": {"id": 42, "username": "alice"},
				"created_at": "2026-03-01T10:00:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123
			}`

			var note Note
			if err := json.Unmarshal([]byte(raw), &note); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if note.Type != tc.wantType {
				t.Fatalf("Type = %q, want %q", note.Type, tc.wantType)
			}
			if note.ID != 456 {
				t.Fatalf("ID = %d, want 456", note.ID)
			}

			out, err := json.Marshal(note)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			var roundTripped Note
			if err := json.Unmarshal(out, &roundTripped); err != nil {
				t.Fatalf("Unmarshal(round-tripped) error = %v", err)
			}
			if roundTripped.Type != tc.wantType {
				t.Errorf("round-tripped Type = %q, want %q", roundTripped.Type, tc.wantType)
			}

			// Compare via re-marshaled JSON rather than struct equality:
			// time.Time is documented as unsafe to compare with ==.
			out2, err := json.Marshal(roundTripped)
			if err != nil {
				t.Fatalf("Marshal(round-tripped) error = %v", err)
			}
			if string(out) != string(out2) {
				t.Errorf("second marshal = %s, want %s", out2, out)
			}
		})
	}
}
