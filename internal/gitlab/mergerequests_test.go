package gitlab

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
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
		name                   string
		call                   func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error)
		path                   string
		wantNonArchivedPresent bool
		wantNonArchivedValue   string
		wantParamKeys          []string
	}{
		{
			"global list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.MergeRequests(t.Context(), w)
			},
			"/api/v4/merge_requests",
			true, "false",
			[]string{"created_before", "non_archived", "page", "per_page", "scope", "updated_after"},
		},
		{
			"group list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.GroupMergeRequests(t.Context(), NumericID(9), w)
			},
			"/api/v4/groups/9/merge_requests",
			true, "false",
			[]string{"created_before", "non_archived", "page", "per_page", "scope", "updated_after"},
		},
		{
			"project list",
			func(c *Client, w MergeRequestWindow) ([]MergeRequestSummary, error) {
				return c.ProjectMergeRequests(t.Context(), NumericID(123), w)
			},
			"/api/v4/projects/123/merge_requests",
			false, "",
			[]string{"created_before", "page", "per_page", "scope", "updated_after"},
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

			gotValue, gotPresent := gotQuery["non_archived"]
			if gotPresent != tc.wantNonArchivedPresent {
				t.Errorf("non_archived present = %v, want %v", gotPresent, tc.wantNonArchivedPresent)
			}
			if tc.wantNonArchivedPresent && (len(gotValue) != 1 || gotValue[0] != tc.wantNonArchivedValue) {
				t.Errorf("non_archived = %v, want literally [%q]", gotValue, tc.wantNonArchivedValue)
			}

			// scope must be sent as literally "all" on every list
			// endpoint: the global GET /merge_requests defaults scope
			// to created_by_me when the param is absent, silently
			// dropping every merge request the token owner did not
			// author (see scopeAll's doc comment). The other two
			// endpoints already default to "all", so this is a no-op
			// there, but it is asserted identically on all three so
			// none of them can regress independently.
			if gotScope := gotQuery.Get("scope"); gotScope != "all" {
				t.Errorf("scope = %q, want literally \"all\"", gotScope)
			}

			// Full param-set assertion, not an allowlist: any param a
			// future change adds, removes, or forgets (such as the
			// missing scope=all that caused the original bug) changes
			// this set and fails here, rather than staying invisible
			// because nobody thought to assert it individually.
			gotKeys := make([]string, 0, len(gotQuery))
			for k := range gotQuery {
				gotKeys = append(gotKeys, k)
			}
			slices.Sort(gotKeys)
			wantKeys := slices.Clone(tc.wantParamKeys)
			slices.Sort(wantKeys)
			if !slices.Equal(gotKeys, wantKeys) {
				t.Errorf("%s query param set = %v, want exactly %v", tc.name, gotKeys, wantKeys)
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

// tokenOwnerAuthorID and colleagueAuthorID are the two merge request
// authors used by the fake server in
// TestClient_MergeRequests_globalListReturnsOtherAuthorsMRWhenNoScopeGiven.
const (
	tokenOwnerAuthorID = 1
	colleagueAuthorID  = 2
)

// fakeGlobalMRScopeServer implements the documented GitLab 18.11 default
// for GET /merge_requests: with scope absent, it returns only merge
// requests authored by the token owner ("created_by_me"); with
// scope=all, it returns every merge request regardless of author. This is
// what the earlier fixture-based fake in this file could never catch --
// that fake ignored the query and returned the same fixture either way,
// so a missing scope param was invisible to it.
func fakeGlobalMRScopeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ownerMR := map[string]any{
			"project_id": 1, "iid": 10, "title": "own work", "web_url": "u1",
			"created_at": "2026-07-01T00:00:00Z", "updated_at": "2026-07-01T00:00:00Z",
			"author":           map[string]any{"id": tokenOwnerAuthorID, "username": "token-owner"},
			"user_notes_count": 0,
		}
		colleagueMR := map[string]any{
			"project_id": 2, "iid": 20, "title": "colleague's MR", "web_url": "u2",
			"created_at": "2026-07-02T00:00:00Z", "updated_at": "2026-07-02T00:00:00Z",
			"author": map[string]any{"id": colleagueAuthorID, "username": "colleague"},
			// Non-zero user_notes_count stands in for the 4 comments the
			// target user left on this merge request: exactly the
			// merge request the reported bug dropped at the list
			// stage, before /discussions was ever fetched.
			"user_notes_count": 4,
		}

		switch r.URL.Query().Get("scope") {
		case "":
			writeJSON(t, w, []map[string]any{ownerMR})
		case "all":
			writeJSON(t, w, []map[string]any{ownerMR, colleagueMR})
		default:
			t.Fatalf("unexpected scope = %q", r.URL.Query().Get("scope"))
		}
	}))
}

// TestClient_MergeRequests_globalListReturnsOtherAuthorsMRWhenNoScopeGiven
// is the behavioral regression test for the reported bug: a user searched
// for a colleague's comments over a date window with no --group/--project,
// and got back only merge requests the TOKEN OWNER had created. The
// colleague's merge request, carrying 4 comments by the target user, was
// dropped at the list stage. Removing the scope=all query.Set call from
// MergeRequestWindow.query must fail this test.
func TestClient_MergeRequests_globalListReturnsOtherAuthorsMRWhenNoScopeGiven(t *testing.T) {
	srv := fakeGlobalMRScopeServer(t)
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.July, 16),
		UpdatedAfter:  domain.NewDate(2026, time.July, 13),
	}
	items, err := c.MergeRequests(t.Context(), window)
	if err != nil {
		t.Fatalf("MergeRequests() error = %v", err)
	}

	var sawColleagueMR bool
	for _, mr := range items {
		if mr.Author.ID == colleagueAuthorID && mr.ProjectID == 2 && mr.IID == 20 {
			sawColleagueMR = true
			if mr.UserNotesCount != 4 {
				t.Errorf("colleague's MR user_notes_count = %d, want 4", mr.UserNotesCount)
			}
		}
	}
	if !sawColleagueMR {
		t.Fatalf("MergeRequests() = %+v, want it to include the colleague-authored merge request carrying the target user's comments (scope=all must be sent, or GitLab's created_by_me default silently drops it)", items)
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

// mrReferencesFixture returns three merge requests: one with a plain
// project path in references.full, one with a nested subgroup path, and
// one with no references object at all.
func mrReferencesFixture() string {
	return `[
		{
			"project_id": 1, "iid": 1, "title": "t1", "web_url": "u1",
			"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			"author": {"id": 1, "username": "a"}, "user_notes_count": 0,
			"references": {"short": "!1", "relative": "!1", "full": "my-group/my-project!1"}
		},
		{
			"project_id": 2, "iid": 42, "title": "t2", "web_url": "u2",
			"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			"author": {"id": 1, "username": "a"}, "user_notes_count": 0,
			"references": {"short": "!42", "relative": "group/subgroup/project!42", "full": "group/subgroup/project!42"}
		},
		{
			"project_id": 3, "iid": 3, "title": "t3", "web_url": "u3",
			"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
			"author": {"id": 1, "username": "a"}, "user_notes_count": 0
		}
	]`
}

// TestClient_MergeRequestLists_populateProjectPathFromReferences proves
// CHANGE 1: on every merge request list endpoint (global, group, project,
// and the project iids[] batch endpoint), ProjectPath is derived from
// references.full at zero extra API cost, a nested subgroup path round
// trips correctly, and a merge request with no references object at all
// yields an empty ProjectPath and no error.
func TestClient_MergeRequestLists_populateProjectPathFromReferences(t *testing.T) {
	window := MergeRequestWindow{
		CreatedBefore: domain.NewDate(2026, time.June, 30),
		UpdatedAfter:  domain.NewDate(2026, time.January, 1),
	}

	cases := []struct {
		name string
		call func(c *Client) ([]MergeRequestSummary, error)
	}{
		{"global list", func(c *Client) ([]MergeRequestSummary, error) {
			return c.MergeRequests(t.Context(), window)
		}},
		{"group list", func(c *Client) ([]MergeRequestSummary, error) {
			return c.GroupMergeRequests(t.Context(), NumericID(9), window)
		}},
		{"project list", func(c *Client) ([]MergeRequestSummary, error) {
			return c.ProjectMergeRequests(t.Context(), NumericID(123), window)
		}},
		{"project iids[] batch", func(c *Client) ([]MergeRequestSummary, error) {
			return c.ProjectMergeRequestsByIIDs(t.Context(), NumericID(123), []int64{1, 42, 3})
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("page") != "1" {
					w.Write([]byte(`[]`))
					return
				}
				w.Write([]byte(mrReferencesFixture()))
			}))
			defer srv.Close()

			c := NewClient(srv.URL, "token")
			items, err := tc.call(c)
			if err != nil {
				t.Fatalf("%s error = %v", tc.name, err)
			}
			if len(items) != 3 {
				t.Fatalf("%s returned %d items, want 3", tc.name, len(items))
			}

			byIID := make(map[int64]MergeRequestSummary, len(items))
			for _, it := range items {
				byIID[it.IID] = it
			}

			if got := byIID[1].ProjectPath; got != "my-group/my-project" {
				t.Errorf("iid=1 ProjectPath = %q, want %q", got, "my-group/my-project")
			}
			if got := byIID[42].ProjectPath; got != "group/subgroup/project" {
				t.Errorf("iid=42 (nested subgroup) ProjectPath = %q, want %q", got, "group/subgroup/project")
			}
			if got := byIID[3].ProjectPath; got != "" {
				t.Errorf("iid=3 (no references) ProjectPath = %q, want empty", got)
			}
		})
	}
}

// mustParseInt64 is a test-only strconv.ParseInt wrapper that fails the
// test instead of returning an error, keeping fixture-building code above
// terse.
func mustParseInt64(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("parse %q as int64: %v", s, err)
	}
	return n
}

// TestClient_ProjectMergeRequestsByIIDs_batchesAndMergesWithoutDuplicates
// proves CHANGE 3: a candidate list longer than iidBatchSize is split into
// multiple requests, each carrying the expected iids[] values, covering
// every iid exactly once with no gaps and no overlap, and the merged
// result contains every iid exactly once.
func TestClient_ProjectMergeRequestsByIIDs_batchesAndMergesWithoutDuplicates(t *testing.T) {
	const total = 2*iidBatchSize + 50
	iids := make([]int64, total)
	for i := range iids {
		iids[i] = int64(i + 1)
	}

	var gotBatches [][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/projects/123/merge_requests" {
			t.Fatalf("path = %q, want /api/v4/projects/123/merge_requests", r.URL.Path)
		}
		if r.URL.Query().Get("page") != "1" {
			w.Write([]byte(`[]`))
			return
		}
		batchIIDs := r.URL.Query()["iids[]"]
		gotBatches = append(gotBatches, batchIIDs)

		items := make([]map[string]any, len(batchIIDs))
		for i, s := range batchIIDs {
			items[i] = map[string]any{
				"project_id": 123, "iid": mustParseInt64(t, s), "title": "t", "web_url": "u",
				"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
				"author": map[string]any{"id": 1, "username": "a"}, "user_notes_count": 0,
			}
		}
		writeJSON(t, w, items)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	items, err := c.ProjectMergeRequestsByIIDs(t.Context(), NumericID(123), iids)
	if err != nil {
		t.Fatalf("ProjectMergeRequestsByIIDs() error = %v", err)
	}

	if len(gotBatches) != 3 {
		t.Fatalf("requests sent = %d, want 3 (2 full batches of %d + 1 of 50)", len(gotBatches), iidBatchSize)
	}

	wantBatch := func(fromIID int64, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strconv.FormatInt(fromIID+int64(i), 10)
		}
		return out
	}
	wantBatches := [][]string{
		wantBatch(1, iidBatchSize),
		wantBatch(iidBatchSize+1, iidBatchSize),
		wantBatch(2*iidBatchSize+1, 50),
	}
	for i, want := range wantBatches {
		if !slices.Equal(gotBatches[i], want) {
			t.Errorf("batch %d iids[] = %v, want %v", i, gotBatches[i], want)
		}
	}

	if len(items) != total {
		t.Fatalf("merged items = %d, want %d", len(items), total)
	}
	seen := make(map[int64]bool, len(items))
	for _, it := range items {
		if seen[it.IID] {
			t.Errorf("duplicate iid %d in merged results", it.IID)
		}
		seen[it.IID] = true
	}
	for _, iid := range iids {
		if !seen[iid] {
			t.Errorf("iid %d missing from merged results", iid)
		}
	}
}
