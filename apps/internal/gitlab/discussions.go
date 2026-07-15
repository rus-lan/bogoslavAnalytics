package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

// Discussions fetches every discussion thread of a merge request via
// GET /projects/:id/merge_requests/:iid/discussions -- the exact-count
// source for TZ.md section 5.4. Counting must never use /notes instead:
// per TZ.md section 5.6.2, that endpoint silently omits DiscussionNote
// replies inside threads.
//
// Parsing is lenient: unknown or extra note fields (TZ.md's own caveat --
// the documented example JSON is GitLab 10.x-era, current instances may
// add undocumented fields) are ignored by domain.Discussion's ordinary
// struct decoding.
//
// If the listing hits the page limit, the discussions collected so far are
// returned together with ErrPageLimitReached.
//
// project may be a numeric id or a namespaced path (ID).
func (c *Client) Discussions(ctx context.Context, project ID, mrIID int64) ([]domain.Discussion, error) {
	path := fmt.Sprintf("/projects/%s/merge_requests/%d/discussions", project.segment(), mrIID)

	items, err := paginate(ctx, func(ctx context.Context, page int) ([]domain.Discussion, error) {
		q := url.Values{}
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
		var pageItems []domain.Discussion
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			return nil, fmt.Errorf("decode discussions page %d: %w", page, err)
		}
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: discussions for project %s mr %d: %w", project, mrIID, err)
	}
	return items, err
}
