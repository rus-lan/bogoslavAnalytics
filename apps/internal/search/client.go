package search

import (
	"context"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// Client is the read-only surface search needs from a GitLab client. It is
// defined on search's own side, not gitlab's, so strategies can be tested
// against a fake instead of a real HTTP server (TZ.md section 2.4).
// *gitlab.Client satisfies it; see the compile-time check below.
type Client interface {
	// CommentEvents fetches userID's comment events inside window -- the
	// candidate source for the events strategy (TZ.md section 5.1).
	CommentEvents(ctx context.Context, userID int64, window domain.DateRange) ([]gitlab.CommentEvent, error)

	// Discussions fetches every discussion thread of a merge request --
	// the exact-count source for both strategies (TZ.md section 5.4).
	Discussions(ctx context.Context, projectID, mrIID int64) ([]domain.Discussion, error)

	// GroupProjects lists every project in a group, including subgroups --
	// used to scope both strategies to a group (TZ.md sections 5.1.6 and
	// 5.2.1).
	GroupProjects(ctx context.Context, groupID int64) ([]domain.Project, error)

	// MergeRequests, GroupMergeRequests and ProjectMergeRequests list
	// merge requests windowed by created_before/updated_after -- the
	// candidate source for the bruteforce strategy, scoped to the whole
	// token, a group, or a single project respectively (TZ.md section
	// 5.2.1).
	MergeRequests(ctx context.Context, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	GroupMergeRequests(ctx context.Context, groupID int64, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)
	ProjectMergeRequests(ctx context.Context, projectID int64, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)

	// SmokeTest reports whether this instance's Events API can be trusted
	// to surface DiscussionNote replies -- the input the autoselector
	// gates events on (TZ.md sections 5.3b and 5.5).
	SmokeTest(ctx context.Context, userID int64) (domain.SmokeResult, error)
}

// var _ Client = (*gitlab.Client)(nil) proves gitlab.Client satisfies the
// interface search defines for itself, without any search function
// signature ever naming the concrete *gitlab.Client type (TZ.md section
// 2.4).
var _ Client = (*gitlab.Client)(nil)
