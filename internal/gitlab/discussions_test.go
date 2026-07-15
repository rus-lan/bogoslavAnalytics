package gitlab

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

const discussionsFixture = `[
	{
		"id": "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"individual_note": true,
		"notes": [
			{
				"id": 100,
				"type": null,
				"body": "plain top-level comment",
				"author": {"id": 42, "username": "alice"},
				"created_at": "2026-03-01T09:00:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123,
				"unexpected_future_field": {"nested": true}
			}
		]
	},
	{
		"id": "b1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		"individual_note": false,
		"notes": [
			{
				"id": 200,
				"type": "DiscussionNote",
				"body": "thread reply",
				"author": {"id": 42, "username": "alice"},
				"created_at": "2026-03-01T10:00:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123
			},
			{
				"id": 201,
				"type": "DiffNote",
				"body": "diff comment",
				"author": {"id": 43, "username": "bob"},
				"created_at": "2026-03-01T10:05:00Z",
				"system": false,
				"noteable_id": 77,
				"noteable_type": "MergeRequest",
				"project_id": 123,
				"commit_id": "deadbeef",
				"position": {
					"base_sha": "aaa",
					"start_sha": "bbb",
					"head_sha": "ccc",
					"old_path": "foo.go",
					"new_path": "foo.go",
					"position_type": "text",
					"old_line": 10,
					"new_line": 12,
					"line_range": {"start": {"line_code": "x"}}
				}
			}
		]
	}
]`

func TestClient_Discussions_parsesDiscussionNoteDiffNoteAndNullType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/notes" {
			t.Fatal("request hit /notes, which must never be used for counting")
		}
		if r.URL.Path != "/api/v4/projects/123/merge_requests/77/discussions" {
			t.Fatalf("path = %q, want /api/v4/projects/123/merge_requests/77/discussions", r.URL.Path)
		}
		w.Write([]byte(discussionsFixture))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	discussions, err := c.Discussions(t.Context(), NumericID(123), 77)
	if err != nil {
		t.Fatalf("Discussions() error = %v", err)
	}
	if len(discussions) != 2 {
		t.Fatalf("Discussions() returned %d discussions, want 2", len(discussions))
	}

	plain := discussions[0].Notes[0]
	if plain.Type != domain.NoteTypeNone {
		t.Errorf("plain note Type = %q, want NoteTypeNone (null)", plain.Type)
	}
	if plain.Author.ID != 42 {
		t.Errorf("plain note Author.ID = %d, want 42", plain.Author.ID)
	}
	if plain.System {
		t.Errorf("plain note System = true, want false")
	}
	if plain.CreatedAt.IsZero() {
		t.Errorf("plain note CreatedAt is zero, want a parsed timestamp")
	}

	reply := discussions[1].Notes[0]
	if reply.Type != domain.NoteTypeDiscussion {
		t.Errorf("reply note Type = %q, want DiscussionNote", reply.Type)
	}
	if reply.Author.ID != 42 {
		t.Errorf("reply note Author.ID = %d, want 42", reply.Author.ID)
	}

	diff := discussions[1].Notes[1]
	if diff.Type != domain.NoteTypeDiff {
		t.Errorf("diff note Type = %q, want DiffNote", diff.Type)
	}
	if diff.Author.ID != 43 {
		t.Errorf("diff note Author.ID = %d, want 43", diff.Author.ID)
	}
	if diff.System {
		t.Errorf("diff note System = true, want false")
	}
	if diff.CreatedAt.IsZero() {
		t.Errorf("diff note CreatedAt is zero, want a parsed timestamp")
	}
}

func TestClient_Discussions_unknownFieldsDoNotBreakParsing(t *testing.T) {
	// The first discussion's first note carries an undocumented field
	// ("unexpected_future_field"); this must be ignored, not fail parsing.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(discussionsFixture))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	discussions, err := c.Discussions(t.Context(), NumericID(123), 77)
	if err != nil {
		t.Fatalf("Discussions() error = %v, want success even with unknown fields present", err)
	}
	if len(discussions) != 2 || len(discussions[0].Notes) != 1 {
		t.Fatalf("Discussions() = %+v, want the fixture's 2 discussions with unknown fields ignored", discussions)
	}
}

func TestClient_Discussions_neverRequestsNotesPath(t *testing.T) {
	var pathsHit []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pathsHit = append(pathsHit, r.URL.Path)
		if r.URL.Path == "/api/v4/notes" || r.URL.Path == "/api/v4/projects/123/merge_requests/77/notes" {
			t.Fatalf("request hit a /notes path (%s), which must never be used for counting", r.URL.Path)
		}
		w.Write([]byte(`[{"id": "x", "individual_note": true, "notes": [{"id": 1, "type": "DiscussionNote", "body": "r", "author": {"id": 1, "username": "a"}, "created_at": "2026-03-01T10:00:00Z", "system": false, "noteable_id": 77, "noteable_type": "MergeRequest", "project_id": 123}]}]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.Discussions(t.Context(), NumericID(123), 77); err != nil {
		t.Fatalf("Discussions() error = %v", err)
	}
	for _, p := range pathsHit {
		if p != "/api/v4/projects/123/merge_requests/77/discussions" {
			t.Errorf("unexpected path hit: %s", p)
		}
	}
}
