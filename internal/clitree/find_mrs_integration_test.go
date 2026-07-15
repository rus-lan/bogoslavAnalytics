package clitree

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindMRs_pointModeEndToEnd runs the real find-mrs command, with a
// real *gitlab.Client, against a fake GitLab server: it proves the whole
// thin-adapter wiring (flags -> request -> app.FindMRs -> client -> HTTP
// -> artifact write -> stdout/--out rendering), not just the flag ->
// request-struct mapping the rest of this package's tests check in
// isolation.
func TestFindMRs_pointModeEndToEnd(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/5/merge_requests", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("iids[]"); got != "9" {
			t.Errorf("iids[] = %q, want 9", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": 1, "iid": 9, "project_id": 5,
			"title": "fix bug", "web_url": "https://gitlab.example.com/my-group/repo/-/merge_requests/9",
			"created_at": "2026-03-01T10:00:00Z", "updated_at": "2026-03-05T10:00:00Z",
			"references": {"full": "my-group/repo!9"},
			"user_notes_count": 3
		}]`))
	})
	mux.HandleFunc("/api/v4/projects/5/merge_requests/9/discussions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id": "abc", "individual_note": false,
			"notes": [
				{"id": 100, "type": "DiscussionNote", "body": "looks good", "author": {"id": 42, "username": "alice"}, "created_at": "2026-03-02T10:00:00Z", "system": false, "noteable_id": 999, "noteable_type": "MergeRequest", "project_id": 5},
				{"id": 101, "type": "DiscussionNote", "body": "please fix", "author": {"id": 42, "username": "alice"}, "created_at": "2026-03-03T10:00:00Z", "system": false, "noteable_id": 999, "noteable_type": "MergeRequest", "project_id": 5}
			]
		}]`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("GITLAB_URL", server.URL)
	t.Setenv("GITLAB_TOKEN", "dummy-token")

	dir := t.TempDir()
	outFile := filepath.Join(dir, "result.yaml")

	cmd := NewRootCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetArgs([]string{
		"find-mrs",
		"--user", "42",
		"--from", "2026-01-01",
		"--to", "2026-06-30",
		"--project", "5",
		"--mr", "9",
		"--artifacts-dir", dir,
		"--out", outFile,
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v, stderr:\n%s", err, stderr.String())
	}

	if !strings.Contains(stderr.String(), "mode: point") {
		t.Errorf("stderr = %q, want it to mention point mode", stderr.String())
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", outFile, err)
	}
	got := string(data)
	if !strings.Contains(got, "comment_count: 2") {
		t.Errorf("--out content = %q, want it to contain comment_count: 2", got)
	}
	if !strings.Contains(got, "mr_iid: 9") {
		t.Errorf("--out content = %q, want it to contain mr_iid: 9", got)
	}
}
