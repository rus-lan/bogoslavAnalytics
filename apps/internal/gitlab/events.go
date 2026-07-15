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

// EventNote is the note object nested inside a comment CommentEvent
// (TZ.md section 5.1.2).
type EventNote struct {
	System       bool   `json:"system"`
	NoteableID   int64  `json:"noteable_id"`
	NoteableType string `json:"noteable_type"`
	NoteableIID  int64  `json:"noteable_iid"`
}

// CommentEvent is a single "commented on" activity event, as returned by
// GET /users/:id/events with action=commented (TZ.md section 5.1.2).
type CommentEvent struct {
	ProjectID  int64     `json:"project_id"`
	ActionName string    `json:"action_name"`
	TargetType string    `json:"target_type"`
	CreatedAt  time.Time `json:"created_at"`
	Note       EventNote `json:"note"`
}

// CommentEvents fetches the comment events for userID inside window,
// implementing the request-side trap and the client-side trap called out
// by TZ.md sections 5.1.1, 5.6.1 and 5.6.5:
//
//   - action=commented is the only filter sent; target_type is never sent,
//     since it silently drops DiscussionNote replies.
//   - after/before are date-only (type "date (ISO 8601)", e.g.
//     "after=2017-01-31", per https://docs.gitlab.com/api/events/; the
//     underlying Grape parameter is declared `type: Date`, so a time
//     component would silently be discarded even if sent) and their
//     inclusivity is undocumented, so the request pads the window by one
//     day on each side and the exact [window.From, window.To] boundary is
//     enforced here, on the client, by dropping any event whose
//     created_at falls outside it.
//
// If the listing hits the page limit, the events collected so far are
// returned together with ErrPageLimitReached.
func (c *Client) CommentEvents(ctx context.Context, userID int64, window domain.DateRange) ([]CommentEvent, error) {
	after := window.From.Start().AddDate(0, 0, -1)
	before := window.To.Start().AddDate(0, 0, 1)
	path := "/users/" + strconv.FormatInt(userID, 10) + "/events"

	events, err := paginate(ctx, func(ctx context.Context, page int) ([]CommentEvent, error) {
		q := url.Values{}
		q.Set("action", "commented")
		q.Set("after", after.Format("2006-01-02"))
		q.Set("before", before.Format("2006-01-02"))
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
		var pageItems []CommentEvent
		if err := json.NewDecoder(resp.Body).Decode(&pageItems); err != nil {
			return nil, fmt.Errorf("decode events page %d: %w", page, err)
		}
		return pageItems, nil
	})
	if err != nil && !errors.Is(err, ErrPageLimitReached) {
		return nil, fmt.Errorf("gitlab: comment events for user %d: %w", userID, err)
	}

	filtered := events[:0]
	for _, e := range events {
		if window.Contains(e.CreatedAt) {
			filtered = append(filtered, e)
		}
	}
	return filtered, err
}
