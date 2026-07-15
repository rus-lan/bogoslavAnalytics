package gitlab

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// ResolveUserID resolves a GitLab username to its numeric id via
// GET /users?username=<name> (TZ.md section 5.0). The response is a JSON
// array of zero or one elements, never a single object and never a 404 on
// a miss; username matching is case-insensitive on the server, so the
// value is passed through unchanged. An empty array returns
// domain.ErrUserNotFound.
func (c *Client) ResolveUserID(ctx context.Context, username string) (int64, error) {
	q := url.Values{}
	q.Set("username", username)

	resp, err := c.request(ctx, http.MethodGet, "/users", q)
	if err != nil {
		return 0, fmt.Errorf("gitlab: resolve user %q: %w", username, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("gitlab: resolve user %q: %w", username, unexpectedStatus(resp))
	}

	var users []struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return 0, fmt.Errorf("gitlab: resolve user %q: decode: %w", username, err)
	}
	if len(users) == 0 {
		return 0, fmt.Errorf("gitlab: resolve user %q: %w", username, domain.ErrUserNotFound)
	}
	return users[0].ID, nil
}
