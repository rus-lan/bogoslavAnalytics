package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClient_GroupProjects_includesSubgroups(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Write([]byte(`[{"id": 1, "path_with_namespace": "my-group/repo"}, {"id": 2, "path_with_namespace": "my-group/sub/other"}]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	projects, err := c.GroupProjects(t.Context(), NumericID(9))
	if err != nil {
		t.Fatalf("GroupProjects() error = %v", err)
	}

	if gotPath != "/api/v4/groups/9/projects" {
		t.Errorf("path = %q, want /api/v4/groups/9/projects", gotPath)
	}
	if got := gotQuery.Get("include_subgroups"); got != "true" {
		t.Errorf("include_subgroups = %q, want true", got)
	}

	if len(projects) != 2 {
		t.Fatalf("GroupProjects() returned %d projects, want 2", len(projects))
	}
	if projects[0].ID != 1 || projects[0].Path != "my-group/repo" {
		t.Errorf("projects[0] = %+v, want {ID: 1, Path: my-group/repo}", projects[0])
	}
	if projects[1].ID != 2 || projects[1].Path != "my-group/sub/other" {
		t.Errorf("projects[1] = %+v, want {ID: 2, Path: my-group/sub/other}", projects[1])
	}
}
