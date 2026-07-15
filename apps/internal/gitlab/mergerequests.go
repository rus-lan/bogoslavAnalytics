package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// MergeRequestWindow is the bruteforce window predicate (TZ.md section
// 5.2.2): created_before=CreatedBefore, updated_after=UpdatedAfter.
// updated_before must never be added -- it would cut out merge requests
// updated after the window.
type MergeRequestWindow struct {
	CreatedBefore domain.Date
	UpdatedAfter  domain.Date
}

// MergeRequestSummary is one merge request as returned by a merge request
// list endpoint, plus the user_notes_count pre-filter field (TZ.md section
// 5.2.3). CommentCount is not populated by a list endpoint; it is filled
// in later from the exact /discussions count.
type MergeRequestSummary struct {
	domain.MergeRequest
	UserNotesCount int `json:"user_notes_count"`
}

// nonArchivedFalse is the literal value sent for the non_archived merge
// request list parameter (18.11 docs). The two endpoints that carry it
// document different DEFAULTS: "GET /groups/:id/merge_requests" defaults
// non_archived to true, while the global "GET /merge_requests" defaults it
// to false. Left unset, the same query (windowed bruteforce over a group
// vs. over the whole token) would silently drop merge requests that live
// in an archived project on one endpoint but not the other -- exactly the
// strategy-dependent silent divergence this project has already been
// bitten by once. Setting it explicitly and identically to "false" (i.e.
// include archived projects) on both endpoints makes bruteforce match what
// the events strategy already sees: a comment a user left is still a
// comment they left, even if the project holding it has since been
// archived.
//
// GET /projects/:id/merge_requests (a single, already-identified project)
// does not carry this parameter at all, so ProjectMergeRequests never sets
// it.
const nonArchivedFalse = "false"

// query builds the shared query parameters for one page of a merge
// request listing. The "view" parameter is never set: leaving it unset
// keeps the default view, which is required for user_notes_count to be
// present (TZ.md section 5.2.3: "view=simple использовать нельзя").
//
// created_before/updated_after are sent as whole-second RFC 3339 with a Z
// offset ("2019-03-15T08:00:00Z"), matching the only documented example
// for these datetime parameters
// (https://docs.gitlab.com/api/merge_requests/). Fractional seconds are
// not part of the documented contract, so the instant is truncated to the
// second before formatting rather than relying on time.RFC3339Nano.
func (w MergeRequestWindow) query(page int) url.Values {
	q := url.Values{}
	q.Set("created_before", formatMRDateTime(w.CreatedBefore.End()))
	q.Set("updated_after", formatMRDateTime(w.UpdatedAfter.Start()))
	q.Set("per_page", strconv.Itoa(perPage))
	q.Set("page", strconv.Itoa(page))
	return q
}

// formatMRDateTime renders t as the whole-second RFC 3339 UTC form the
// merge request list endpoints document, e.g. "2019-03-15T08:00:00Z".
func formatMRDateTime(t time.Time) string {
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

// MergeRequests lists merge requests across every project visible to the
// token via GET /merge_requests (TZ.md section 5.2.1). non_archived=false
// is always sent explicitly (see nonArchivedFalse).
func (c *Client) MergeRequests(ctx context.Context, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, "/merge_requests", window, true)
}

// GroupMergeRequests lists merge requests scoped to a group via
// GET /groups/:id/merge_requests (TZ.md section 5.2.1). group may be a
// numeric id or a namespaced path (ID). non_archived=false is always sent
// explicitly (see nonArchivedFalse).
func (c *Client) GroupMergeRequests(ctx context.Context, group ID, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, fmt.Sprintf("/groups/%s/merge_requests", group.segment()), window, true)
}

// ProjectMergeRequests lists merge requests scoped to a project via
// GET /projects/:id/merge_requests (TZ.md section 5.2.1). project may be a
// numeric id or a namespaced path (ID). Unlike MergeRequests and
// GroupMergeRequests, non_archived is never sent here: it is not a
// documented parameter of this endpoint (see nonArchivedFalse).
func (c *Client) ProjectMergeRequests(ctx context.Context, project ID, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, fmt.Sprintf("/projects/%s/merge_requests", project.segment()), window, false)
}

func (c *Client) listMergeRequests(ctx context.Context, path string, window MergeRequestWindow, setNonArchived bool) ([]MergeRequestSummary, error) {
	items, err := paginate(ctx, func(ctx context.Context, page int) ([]MergeRequestSummary, error) {
		q := window.query(page)
		if setNonArchived {
			q.Set("non_archived", nonArchivedFalse)
		}

		resp, err := c.request(ctx, http.MethodGet, path, q)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, unexpectedStatus(resp)
		}
		var pageItems []MergeRequestSummary
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			return nil, fmt.Errorf("decode merge requests page %d: %w", page, err)
		}
		populateProjectPaths(pageItems)
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: merge requests %s: %w", path, err)
	}
	return items, err
}

// populateProjectPaths fills in each item's ProjectPath from its
// References.full (18.11 docs: "references.full ... Complete reference to
// a merge request, including full project path, like
// gitlab-org/gitlab!123"). Every merge request list response already
// carries references, so this closes the gap where artifact.MRItem's
// ProjectPath had no producer (TZ.md section 4.2) at zero extra API cost.
// An item whose references object is absent or empty gets ProjectPath ==
// "" and no error, since References.ProjectPath already handles that case.
func populateProjectPaths(items []MergeRequestSummary) {
	for i := range items {
		items[i].ProjectPath = items[i].References.ProjectPath()
	}
}

const (
	// iidBatchSize bounds how many iids[] values one request to
	// GET /projects/:id/merge_requests carries. GitLab documents that an
	// overly long URL-encoded query string returns 414 Request-URI Too
	// Large without naming a byte ceiling, so this package picks a size
	// instead of risking one giant URL for an arbitrarily long candidate
	// list:
	//
	//   - Capping at perPage (100) means a single batch can never need more
	//     than one full page of results: a project has at most one merge
	//     request per iid, so up to 100 iids can produce at most 100
	//     matches, which is exactly what one page already holds.
	//   - Even at 100 seven-digit iids ("iids[]=1234567&" repeated 100
	//     times, roughly 1.6 KB), the resulting query string stays far
	//     under the smallest URL/header limits ordinary deployments carry
	//     by default (for example nginx's default
	//     large_client_header_buffers of 8k, or Apache's default
	//     LimitRequestLine of 8190), so it needs no per-deployment tuning.
	iidBatchSize = perPage
)

// ProjectMergeRequestsByIIDs fetches the merge requests of project matching
// iids via GET /projects/:id/merge_requests?iids[]=.. (18.11 docs: "iids[]
// | integer array | No | Returns merge requests matching the provided
// IIDs."). project may be a numeric id or a namespaced path (ID).
//
// iids[] is documented ONLY on this project-scoped list -- grepping the
// 18.11 page confirms it does not exist on GET /groups/:id/merge_requests
// nor on the global GET /merge_requests -- so batch enrichment is
// necessarily one call per project, never one call across an entire scope.
// iids is chunked into groups of iidBatchSize per outgoing request (see
// its doc comment for why that size).
//
// This is what lets the events strategy -- whose candidates carry only
// ProjectID/IID (TZ.md section 14) -- fill in the rest of a
// domain.MergeRequest at a fraction of the calls bruteforce spends doing
// the same thing one list page at a time. There is deliberately no
// single-MR-fetch method: this batching already covers that need.
func (c *Client) ProjectMergeRequestsByIIDs(ctx context.Context, project ID, iids []int64) ([]MergeRequestSummary, error) {
	var all []MergeRequestSummary
	for start := 0; start < len(iids); start += iidBatchSize {
		end := min(start+iidBatchSize, len(iids))

		batch, err := c.mergeRequestsByIIDsBatch(ctx, project, iids[start:end])
		all = append(all, batch...)
		if err != nil {
			return all, err
		}
	}
	return all, nil
}

func (c *Client) mergeRequestsByIIDsBatch(ctx context.Context, project ID, iids []int64) ([]MergeRequestSummary, error) {
	path := fmt.Sprintf("/projects/%s/merge_requests", project.segment())

	items, err := paginate(ctx, func(ctx context.Context, page int) ([]MergeRequestSummary, error) {
		q := url.Values{}
		for _, iid := range iids {
			q.Add("iids[]", strconv.FormatInt(iid, 10))
		}
		q.Set("per_page", strconv.Itoa(perPage))
		q.Set("page", strconv.Itoa(page))

		resp, err := c.request(ctx, http.MethodGet, path, q)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, unexpectedStatus(resp)
		}
		var pageItems []MergeRequestSummary
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			return nil, fmt.Errorf("decode merge requests by iids page %d: %w", page, err)
		}
		populateProjectPaths(pageItems)
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: merge requests for project %s by iids: %w", project, err)
	}
	return items, err
}
