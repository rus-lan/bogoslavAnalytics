package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// projectWire is the subset of a GitLab project object GetProject needs to
// build a domain.Project, per the 18.11 archive docs' "Retrieve a project"
// endpoint. The documented example response is not exhaustive, so decoding
// into this named-field struct leaves any other field it carries ignored
// rather than failing.
type projectWire struct {
	ID                int64  `json:"id"`
	PathWithNamespace string `json:"path_with_namespace"`
}

// GetProject retrieves a single project via GET /projects/:id (18.11
// archive docs, "Retrieve a project": "id | integer or string | Yes | The
// ID or URL-encoded path of the project."). project may be a numeric id or
// a namespaced path (ID). A 404 response returns ErrProjectNotFound rather
// than a zero-value domain.Project with a nil error.
func (c *Client) GetProject(ctx context.Context, project ID) (domain.Project, error) {
	path := fmt.Sprintf("/projects/%s", project.segment())

	resp, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return domain.Project{}, fmt.Errorf("gitlab: get project %s: %w", project, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.Project{}, fmt.Errorf("gitlab: get project %s: %w", project, ErrProjectNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return domain.Project{}, fmt.Errorf("gitlab: get project %s: %w", project, unexpectedStatus(resp))
	}

	var wire projectWire
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return domain.Project{}, fmt.Errorf("gitlab: get project %s: decode: %w", project, err)
	}
	return domain.Project{ID: wire.ID, Path: wire.PathWithNamespace}, nil
}
