package gitlab

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// groupProjectWire is the subset of a GitLab project object needed to
// build a domain.Project. path_with_namespace is not literally quoted in
// TZ.md or findings.md (only the endpoint and its include_subgroups=true
// parameter are); it is GitLab's stable field for a project's namespaced
// path, matching domain.Project's own doc comment ("my-group/repo").
type groupProjectWire struct {
	ID                int64  `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// GroupProjects lists every project in a group, including subgroups, via
// GET /groups/:id/projects?include_subgroups=true (TZ.md sections 5.1.6
// and 5.2.1).
func (c *Client) GroupProjects(ctx context.Context, groupID int64) ([]domain.Project, error) {
	path := fmt.Sprintf("/groups/%d/projects", groupID)

	items, err := paginate(ctx, func(ctx context.Context, page int) ([]groupProjectWire, error) {
		q := url.Values{}
		q.Set("include_subgroups", "true")
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
		var pageItems []groupProjectWire
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			return nil, fmt.Errorf("decode group projects page %d: %w", page, err)
		}
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: projects for group %d: %w", groupID, err)
	}

	projects := make([]domain.Project, len(items))
	for i, it := range items {
		projects[i] = domain.Project{ID: it.ID, Path: it.PathWithNamespace}
	}
	return projects, err
}
