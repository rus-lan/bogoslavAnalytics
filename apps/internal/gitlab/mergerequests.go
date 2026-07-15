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
// token via GET /merge_requests (TZ.md section 5.2.1).
func (c *Client) MergeRequests(ctx context.Context, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, "/merge_requests", window)
}

// GroupMergeRequests lists merge requests scoped to a group via
// GET /groups/:id/merge_requests (TZ.md section 5.2.1).
func (c *Client) GroupMergeRequests(ctx context.Context, groupID int64, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, fmt.Sprintf("/groups/%d/merge_requests", groupID), window)
}

// ProjectMergeRequests lists merge requests scoped to a project via
// GET /projects/:id/merge_requests (TZ.md section 5.2.1).
func (c *Client) ProjectMergeRequests(ctx context.Context, projectID int64, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	return c.listMergeRequests(ctx, fmt.Sprintf("/projects/%d/merge_requests", projectID), window)
}

func (c *Client) listMergeRequests(ctx context.Context, path string, window MergeRequestWindow) ([]MergeRequestSummary, error) {
	items, err := paginate(ctx, func(ctx context.Context, page int) ([]MergeRequestSummary, error) {
		resp, err := c.request(ctx, http.MethodGet, path, window.query(page))
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
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: merge requests %s: %w", path, err)
	}
	return items, err
}
