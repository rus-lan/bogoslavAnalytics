package gitlab

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestClient_GetProject_pathEncodesSlashOnRawURI proves a project path
// given via PathID produces a raw outgoing request URI with "/"
// percent-encoded as "%2F", asserted on the literal request line the fake
// server receives (http.Request.RequestURI) rather than a re-parsed
// net/url.URL, which would silently decode "%2F" back to "/" (see (ID).segment).
func TestClient_GetProject_pathEncodesSlashOnRawURI(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`{"id": 7, "path_with_namespace": "my-group/my-project"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GetProject(t.Context(), PathID("my-group/my-project")); err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/projects/my-group%2Fmy-project") {
		t.Errorf("raw request URI = %q, want it to contain /projects/my-group%%2Fmy-project", gotRequestURI)
	}
	if strings.Contains(gotRequestURI, "my-group/my-project") {
		t.Errorf("raw request URI = %q, must not contain an unescaped slash inside the path segment", gotRequestURI)
	}
}

// TestClient_GetProject_numericIDUnencoded proves a numeric id is passed
// through as plain decimal digits, unencoded, on the raw wire.
func TestClient_GetProject_numericIDUnencoded(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`{"id": 42, "path_with_namespace": "my-group/my-project"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GetProject(t.Context(), NumericID(42)); err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/projects/42") {
		t.Errorf("raw request URI = %q, want it to contain /projects/42 verbatim", gotRequestURI)
	}
	if strings.Contains(gotRequestURI, "%") {
		t.Errorf("raw request URI = %q, a numeric id must never be percent-encoded", gotRequestURI)
	}
}

// TestClient_GetProject_nestedSubgroupEncodesBothSlashes proves a nested
// "group/subgroup/project" path has every "/" percent-encoded, not just
// the first one.
func TestClient_GetProject_nestedSubgroupEncodesBothSlashes(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`{"id": 7, "path_with_namespace": "group/subgroup/project"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GetProject(t.Context(), PathID("group/subgroup/project")); err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/projects/group%2Fsubgroup%2Fproject") {
		t.Errorf("raw request URI = %q, want group%%2Fsubgroup%%2Fproject (both slashes encoded)", gotRequestURI)
	}
}

// TestClient_GetProject_notFoundReturnsErrProjectNotFound proves a 404
// response surfaces as ErrProjectNotFound, not a zero-value domain.Project
// with a nil error.
func TestClient_GetProject_notFoundReturnsErrProjectNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "404 Project Not Found"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	_, err := c.GetProject(t.Context(), PathID("my-group/missing-project"))
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("GetProject() error = %v, want wrapping ErrProjectNotFound", err)
	}
}

// TestClient_GetProject_unknownFieldsDoNotBreakParsing proves extra,
// undocumented response fields are ignored rather than failing decode: the
// 18.11 docs' example response is not exhaustive.
func TestClient_GetProject_unknownFieldsDoNotBreakParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"id": 3,
			"description": null,
			"name": "my-project",
			"name_with_namespace": "My Group / My Project",
			"path": "my-project",
			"path_with_namespace": "my-group/my-project",
			"created_at": "2013-09-30T13:46:02Z",
			"default_branch": "main",
			"tag_list": ["example", "disco"],
			"topics": ["example", "disco"],
			"ssh_url_to_repo": "git@example.com:my-group/my-project.git",
			"http_url_to_repo": "http://example.com/my-group/my-project.git",
			"web_url": "http://example.com/my-group/my-project",
			"readme_url": "http://example.com/my-group/my-project/blob/main/README.md",
			"forks_count": 0,
			"avatar_url": null,
			"star_count": 0,
			"unexpected_future_field": {"nested": true}
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	project, err := c.GetProject(t.Context(), NumericID(3))
	if err != nil {
		t.Fatalf("GetProject() error = %v, want success even with unknown fields present", err)
	}
	if project.ID != 3 || project.Path != "my-group/my-project" {
		t.Errorf("GetProject() = %+v, want {ID: 3, Path: my-group/my-project}", project)
	}
}

// TestClient_GetProject_readsIDAndPathWithNamespace proves id and
// path_with_namespace are read from the response's top level, not nested
// or renamed.
func TestClient_GetProject_readsIDAndPathWithNamespace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id": 123, "path_with_namespace": "diaspora/diaspora"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	project, err := c.GetProject(t.Context(), NumericID(123))
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if project.ID != 123 {
		t.Errorf("project.ID = %d, want 123", project.ID)
	}
	if project.Path != "diaspora/diaspora" {
		t.Errorf("project.Path = %q, want diaspora/diaspora", project.Path)
	}
}
