package domain

import (
	"encoding/json"
	"testing"
)

func TestReferences_ProjectPath(t *testing.T) {
	cases := []struct {
		name string
		full string
		want string
	}{
		{"simple project", "my-group/my-project!1", "my-group/my-project"},
		{"nested subgroup", "group/subgroup/project!42", "group/subgroup/project"},
		{"multi-digit iid", "my-group/my-project!123", "my-group/my-project"},
		{"empty full", "", ""},
		{"no exclamation mark", "my-group/my-project", ""},
		{"exclamation mark appears more than once", "my-group/my!weird-project!7", "my-group/my!weird-project"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := References{Full: tc.full}
			if got := r.ProjectPath(); got != tc.want {
				t.Errorf("ProjectPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReferences_ProjectPath_absent(t *testing.T) {
	var r References
	if got := r.ProjectPath(); got != "" {
		t.Errorf("ProjectPath() on zero value = %q, want empty", got)
	}
}

// TestMergeRequest_JSONRoundTrip_references locks in that "references"
// unmarshals into MergeRequest.References, and that "project_path" is
// omitted from marshaled output when ProjectPath is empty but present
// when it is set.
func TestMergeRequest_JSONRoundTrip_references(t *testing.T) {
	cases := []struct {
		name            string
		raw             string
		wantFull        string
		wantPath        string
		wantKeyInOutput bool
	}{
		{
			name: "references present, project_path not yet derived",
			raw: `{
				"project_id": 1,
				"iid": 1,
				"title": "t",
				"web_url": "https://gitlab.example.com/my-group/my-project/-/merge_requests/1",
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
				"author": {"id": 1, "username": "alice"},
				"comment_count": 0,
				"references": {"short": "!1", "relative": "!1", "full": "my-group/my-project!1"}
			}`,
			wantFull:        "my-group/my-project!1",
			wantPath:        "",
			wantKeyInOutput: false,
		},
		{
			name: "references absent from input",
			raw: `{
				"project_id": 1,
				"iid": 1,
				"title": "t",
				"web_url": "https://gitlab.example.com/my-group/my-project/-/merge_requests/1",
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
				"author": {"id": 1, "username": "alice"},
				"comment_count": 0
			}`,
			wantFull:        "",
			wantPath:        "",
			wantKeyInOutput: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mr MergeRequest
			if err := json.Unmarshal([]byte(tc.raw), &mr); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if mr.References.Full != tc.wantFull {
				t.Errorf("References.Full = %q, want %q", mr.References.Full, tc.wantFull)
			}
			if mr.ProjectPath != tc.wantPath {
				t.Errorf("ProjectPath = %q, want %q", mr.ProjectPath, tc.wantPath)
			}

			out, err := json.Marshal(mr)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(out, &doc); err != nil {
				t.Fatalf("Unmarshal(marshaled output) error = %v", err)
			}
			_, hasKey := doc["project_path"]
			if hasKey != tc.wantKeyInOutput {
				t.Errorf("output has project_path key = %v, want %v (output: %s)", hasKey, tc.wantKeyInOutput, out)
			}
		})
	}
}

// TestMergeRequest_JSONRoundTrip_projectPathSet locks in that
// project_path IS present in marshaled output once it is set, matching
// the omitted case above.
func TestMergeRequest_JSONRoundTrip_projectPathSet(t *testing.T) {
	mr := MergeRequest{
		ProjectID:   1,
		ProjectPath: "my-group/my-project",
		IID:         1,
	}
	out, err := json.Marshal(mr)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("Unmarshal(marshaled output) error = %v", err)
	}
	got, ok := doc["project_path"]
	if !ok {
		t.Fatalf("output missing project_path key, output: %s", out)
	}
	if string(got) != `"my-group/my-project"` {
		t.Errorf("project_path = %s, want %q", got, "my-group/my-project")
	}
}
