package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// wholeSecondRFC3339Pattern matches exactly "YYYY-MM-DDTHH:MM:SSZ": the
// only documented form for the merge request list datetime filters
// (https://docs.gitlab.com/api/merge_requests/, e.g.
// "2019-03-15T08:00:00Z"). No fractional seconds.
var wholeSecondRFC3339Pattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)

func mrFixture() string {
	return `[{
		"project_id": 123,
		"iid": 77,
		"title": "fix bug",
		"web_url": "https://gitlab.example.com/my-group/repo/-/merge_requests/77",
		"created_at": "2026-03-01T10:00:00Z",
		"updated_at": "2026-03-05T10:00:00Z",
		"author": {"id": 42, "username": "alice"},
		"user_notes_count": 8
	}]`
}

func TestClient_MergeRequests_windowPredicateParams(t *testing.T) {
	cases := []struct {
		name string
		call func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error)
		path string
	}{
		{
			"global list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.MergeRequests(t.Context(), w)
			},
			"/api/v4/merge_requests",
		},
		{
			"group list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.GroupMergeRequests(t.Context(), 9, w)
			},
			"/api/v4/groups/9/merge_requests",
		},
		{
			"project list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.ProjectMergeRequests(t.Context(), 123, w)
			},
			"/api/v4/projects/123/merge_requests",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			var gotQuery url.Values
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotQuery = r.URL.Query()
				w.Write([]byte(mrFixture()))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "token")
			window := MergeRequestWindow{
				CreatedBefore: domain.NewDate(2026, time.June, 30),
				UpdatedAfter:  domain.NewDate(2026, time.January, 1),
			}
			items, err := tc.call(c, window)
			if err != nil {
				t.Fatalf("%s error = %v", tc.name, err)
			}
			if gotPath != tc.path {
				t.Errorf("path = %q, want %q", gotPath, tc.path)
			}

			if _, present := gotQuery["updated_before"]; present {
				t.Errorf("updated_before was sent = %v, must never be sent (TZ.md section 5.2.2)", gotQuery["updated_before"])
			}
			if gotQuery.Get("created_before") == "" {
				t.Error("created_before was not sent")
			}
			if gotQuery.Get("updated_after") == "" {
				t.Error("updated_after was not sent")
			}
			if _, present := gotQuery["view"]; present {
				t.Errorf("view was sent = %v, must never be set to simple (or at all)", gotQuery["view"])
			}
			if gotQuery.Get("per_page") != "100" {
				t.Errorf("per_page = %q, want 100", gotQuery.Get("per_page"))
			}

			if len(items) != 1 {
				t.Fatalf("returned %d items, want 1", len(items))
			}
			mr := items[0]
			if mr.ProjectID != 123 || mr.IID != 77 {
				t.Errorf("MergeRequestSummary = %+v, want project_id=123 iid=77", mr)
			}
			if mr.UserNotesCount != 8 {
				t.Errorf("UserNotesCount = %d, want 8", mr.UserNotesCount)
			}
			if mr.Author.ID != 42 {
				t.Errorf("Author.ID = %d, want 42", mr.Author.ID)
			}
		})
	}
}

func TestClient_MergeRequests_createdBeforeIsEndOfDayUpdatedAfterIsStartOfDay(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.June, 30),
		UpdatedAfter:  domain.NewDate(2026, time.January, 1),
	}
	if _, err := c.MergeRequests(t.Context(), window); err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}

	createdBefore, err := time.Parse(time.RFC3339, gotQuery.Get("created_before"))
	if err != nil {
		t.Fatalf("parse created_before %q: %v", gotQuery.Get("created_before"), err)
	}
	if createdBefore.UTC().Hour() != 23 || createdBefore.UTC().Minute() != 59 || createdBefore.UTC().Second() != 59 {
		t.Errorf("created_before = %v, want end-of-day (23:59:59, truncated to whole seconds)", createdBefore)
	}

	updatedAfter, err := time.Parse(time.RFC3339, gotQuery.Get("updated_after"))
	if err != nil {
		t.Fatalf("parse updated_after %q: %v", gotQuery.Get("updated_after"), err)
	}
	if !updatedAfter.UTC().Equal(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("updated_after = %v, want start-of-day midnight", updatedAfter)
	}
}

func TestClient_MergeRequests_sendsWholeSecondRFC3339NoFraction(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.June, 30),
		UpdatedAfter:  domain.NewDate(2026, time.January, 1),
	}
	if _, err := c.MergeRequests(t.Context(), window); err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}

	createdBefore := gotQuery.Get("created_before")
	updatedAfter := gotQuery.Get("updated_after")

	if !wholeSecondRFC3339Pattern.MatchString(createdBefore) {
		t.Errorf("created_before = %q, want whole-second RFC 3339 with Z, no fractional part", createdBefore)
	}
	if !wholeSecondRFC3339Pattern.MatchString(updatedAfter) {
		t.Errorf("updated_after = %q, want whole-second RFC 3339 with Z, no fractional part", updatedAfter)
	}

	// Exact literal values, so a future refactor back to RFC3339Nano (or
	// any other fractional representation) fails this test outright.
	if createdBefore != "2026-06-30T23:59:59Z" {
		t.Errorf("created_before = %q, want exactly 2026-06-30T23:59:59Z", createdBefore)
	}
	if updatedAfter != "2026-01-01T00:00:00Z" {
		t.Errorf("updated_after = %q, want exactly 2026-01-01T00:00:00Z", updatedAfter)
	}
}

func TestClient_MergeRequests_parsesBothFractionalAndWholeSecondTimestamps(t *testing.T) {
	// GitLab is inconsistent about fractional seconds in response
	// timestamps within the same documented example: created_at carries
	// none, merged_at carries milliseconds. Both forms must parse.
	body := `[{
		"project_id": 123,
		"iid": 77,
		"title": "fix bug",
		"web_url": "https://gitlab.example.com/my-group/repo/-/merge_requests/77",
		"created_at": "2026-03-01T10:00:00Z",
		"updated_at": "2026-03-05T10:00:00.520Z",
		"author": {"id": 42, "username": "alice"},
		"user_notes_count": 8
	}]`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.June, 30),
		UpdatedAfter:  domain.NewDate(2026, time.January, 1),
	}
	items, err := c.MergeRequests(t.Context(), window)
	if err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	mr := items[0]

	wantCreated := time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC)
	if !mr.CreatedAt.Equal(wantCreated) {
		t.Errorf("CreatedAt (whole-second form) = %v, want %v", mr.CreatedAt, wantCreated)
	}

	wantUpdated := time.Date(2026, time.March, 5, 10, 0, 0, 520_000_000, time.UTC)
	if !mr.UpdatedAt.Equal(wantUpdated) {
		t.Errorf("UpdatedAt (millisecond-fraction form) = %v, want %v", mr.UpdatedAt, wantUpdated)
	}
}

func TestClient_MergeRequests_pagination(t *testing.T) {
	var gotPages []string
	pageOne := make([]map[string]any, perPage)
	for i := range pageOne {
		pageOne[i] = map[string]any{
			"project_id": 1, "iid": i + 1, "title": "t", "web_url": "u",
			"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			"author": map[string]any{"id": 1, "username": "a"}, "user_notes_count": 0,
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		gotPages = append(gotPages, page)
		if page == "1" {
			writeJSON(t, w, pageOne)
			return
		}
		writeJSON(t, w, []map[string]any{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.June, 30),
		UpdatedAfter:  domain.NewDate(2026, time.January, 1),
	}
	items, err := c.MergeRequests(t.Context(), window)
	if err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}
	if len(items) != perPage {
		t.Errorf("items = %d, want %d", len(items), perPage)
	}
	if len(gotPages) != 2 || gotPages[0] != "1" || gotPages[1] != "2" {
		t.Errorf("pages requested = %v, want [1 2]", gotPages)
	}
}
