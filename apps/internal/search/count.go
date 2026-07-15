package search

import (
	"context"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// CountComments returns userID's exact comment count on one merge request
// inside r: the number of entries across every discussion's notes[] where
// author.id == userID, system == false, and created_at falls inside r
// (TZ.md section 5.4). It is the sole source of comment_count for both
// search strategies and must never be replaced by a count over /notes,
// which silently omits DiscussionNote replies.
//
// projectID is always numeric here: every candidate a strategy builds
// carries domain.MergeRequest.ProjectID straight from an events or merge
// request list response, and that field is always numeric (TZ.md sections
// 5.1.2, 5.2.1), so it is wrapped with gitlab.NumericID before being handed
// to Client.Discussions.
func CountComments(ctx context.Context, client Client, projectID, mrIID, userID int64, r domain.DateRange) (int, error) {
	discussions, err := client.Discussions(ctx, gitlab.NumericID(projectID), mrIID)
	if err != nil {
		return 0, fmt.Errorf("search: count comments project %d mr %d: %w", projectID, mrIID, err)
	}

	count := 0
	for _, d := range discussions {
		for _, n := range d.Notes {
			if n.Author.ID != userID || n.System {
				continue
			}
			if !r.Contains(n.CreatedAt) {
				continue
			}
			count++
		}
	}
	return count, nil
}
