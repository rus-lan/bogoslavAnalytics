package gitlab

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestID_PathID_rawRequestURIEncodesSlash proves CHANGE 4: a project path
// given via PathID produces a raw outgoing request URI with "/"
// percent-encoded as "%2F", asserted on the literal request line the fake
// server receives (http.Request.RequestURI) rather than on a
// re-parsed/re-escaped net/url.URL, which is exactly the kind of check
// that would hide a %2F-decoding regression.
func TestID_PathID_rawRequestURIEncodesSlash(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GroupProjects(t.Context(), PathID("my-group/my-project")); err != nil {
		t.Fatalf("GroupProjects() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/groups/my-group%2Fmy-project/projects") {
		t.Errorf("raw request URI = %q, want it to contain /groups/my-group%%2Fmy-project/projects", gotRequestURI)
	}
	if strings.Contains(gotRequestURI, "my-group/my-project") {
		t.Errorf("raw request URI = %q, must not contain an unescaped slash inside the path segment", gotRequestURI)
	}
}

// TestID_NumericID_rawRequestURIUnencoded proves a numeric id is passed
// through as plain decimal digits, unencoded, on the raw wire.
func TestID_NumericID_rawRequestURIUnencoded(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GroupProjects(t.Context(), NumericID(42)); err != nil {
		t.Fatalf("GroupProjects() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/groups/42/projects") {
		t.Errorf("raw request URI = %q, want it to contain /groups/42/projects verbatim", gotRequestURI)
	}
	if strings.Contains(gotRequestURI, "%") {
		t.Errorf("raw request URI = %q, a numeric id must never be percent-encoded", gotRequestURI)
	}
}

// TestID_PathID_nestedSubgroupEncodesBothSlashes proves a nested
// "group/subgroup/project" path has every "/" percent-encoded, not just
// the first one.
func TestID_PathID_nestedSubgroupEncodesBothSlashes(t *testing.T) {
	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	if _, err := c.GroupProjects(t.Context(), PathID("group/subgroup/project")); err != nil {
		t.Fatalf("GroupProjects() error = %v", err)
	}

	if !strings.Contains(gotRequestURI, "/groups/group%2Fsubgroup%2Fproject/projects") {
		t.Errorf("raw request URI = %q, want group%%2Fsubgroup%%2Fproject (both slashes encoded)", gotRequestURI)
	}
}

// TestID_String reports the human-readable form used in error messages:
// decimal digits for a numeric id, the plain path for a path id.
func TestID_String(t *testing.T) {
	if got := NumericID(9).String(); got != "9" {
		t.Errorf("NumericID(9).String() = %q, want %q", got, "9")
	}
	if got := PathID("my-group/my-project").String(); got != "my-group/my-project" {
		t.Errorf("PathID(...).String() = %q, want %q", got, "my-group/my-project")
	}
}
