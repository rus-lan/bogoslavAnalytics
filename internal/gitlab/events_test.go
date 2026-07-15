package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// dateOnlyPattern matches exactly YYYY-MM-DD: the Events API's after/before
// parameters are declared `type: Date` server-side
// (https://docs.gitlab.com/api/events/), so anything carrying a "T" time
// component would be silently truncated by GitLab and must never be sent.
var dateOnlyPattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func TestClient_CommentEvents_neverSendsTargetType(t *testing.T) {
	var gotQueries []map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQueries = append(gotQueries, map[string][]string(r.URL.Query()))
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	from := domain.NewDate(2026, time.January, 1)
	to := domain.NewDate(2026, time.January, 31)
	window, err := domain.NewDateRange(from, to)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	if _, err := c.CommentEvents(t.Context(), 42, window); err != nil {
		t.Fatalf("CommentEvents() error = %v", err)
	}

	if len(gotQueries) == 0 {
		t.Fatal("no request was made")
	}
	for _, q := range gotQueries {
		if _, present := q["target_type"]; present {
			t.Errorf("outgoing query contains target_type = %v, must never be sent", q["target_type"])
		}
	}
}

func TestClient_CommentEvents_requestParams(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	from := domain.NewDate(2026, time.March, 10)
	to := domain.NewDate(2026, time.March, 20)
	window, err := domain.NewDateRange(from, to)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	if _, err := c.CommentEvents(t.Context(), 7, window); err != nil {
		t.Fatalf("CommentEvents() error = %v", err)
	}

	if gotPath != "/api/v4/users/7/events" {
		t.Errorf("path = %q, want /api/v4/users/7/events", gotPath)
	}
	if got := gotQuery.Get("action"); got != "commented" {
		t.Errorf("action = %q, want commented", got)
	}
	if got := gotQuery.Get("after"); got != "2026-03-09" {
		t.Errorf("after = %q, want 2026-03-09 (from padded by -1 day)", got)
	}
	if got := gotQuery.Get("before"); got != "2026-03-21" {
		t.Errorf("before = %q, want 2026-03-21 (to padded by +1 day)", got)
	}
	if got := gotQuery.Get("per_page"); got != "100" {
		t.Errorf("per_page = %q, want 100", got)
	}
}

func TestClient_CommentEvents_sendsDateOnlyAfterBefore(t *testing.T) {
	var gotAfter, gotBefore string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAfter = r.URL.Query().Get("after")
		gotBefore = r.URL.Query().Get("before")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	from := domain.NewDate(2026, time.March, 10)
	to := domain.NewDate(2026, time.March, 20)
	window, err := domain.NewDateRange(from, to)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	if _, err := c.CommentEvents(t.Context(), 7, window); err != nil {
		t.Fatalf("CommentEvents() error = %v", err)
	}

	if !dateOnlyPattern.MatchString(gotAfter) {
		t.Errorf("after = %q, want a bare YYYY-MM-DD date, not a datetime", gotAfter)
	}
	if !dateOnlyPattern.MatchString(gotBefore) {
		t.Errorf("before = %q, want a bare YYYY-MM-DD date, not a datetime", gotBefore)
	}
}

func TestClient_CommentEvents_filtersToExactWindowOnClient(t *testing.T) {
	// The fake server returns events across a wider span than the exact
	// window, simulating undocumented after/before inclusivity: the
	// client must drop everything outside [from, to] itself.
	body := `[
		{"project_id": 1, "action_name": "commented on", "target_type": "DiscussionNote", "created_at": "2026-03-09T23:00:00Z", "note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 5}},
		{"project_id": 1, "action_name": "commented on", "target_type": "DiscussionNote", "created_at": "2026-03-10T00:00:00Z", "note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 5}},
		{"project_id": 1, "action_name": "commented on", "target_type": "Note", "created_at": "2026-03-15T12:00:00Z", "note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 5}},
		{"project_id": 1, "action_name": "commented on", "target_type": "DiscussionNote", "created_at": "2026-03-20T23:59:59.999Z", "note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 5}},
		{"project_id": 1, "action_name": "commented on", "target_type": "DiscussionNote", "created_at": "2026-03-21T00:00:01Z", "note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 5}}
	]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") != "1" {
			w.Write([]byte(`[]`))
			return
		}
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	from := domain.NewDate(2026, time.March, 10)
	to := domain.NewDate(2026, time.March, 20)
	window, err := domain.NewDateRange(from, to)
	if err != nil {
		t.Fatalf("NewDateRange() error = %v", err)
	}

	events, err := c.CommentEvents(t.Context(), 7, window)
	if err != nil {
		t.Fatalf("CommentEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("CommentEvents() returned %d events, want 3 (inside [from, to])", len(events))
	}
}
