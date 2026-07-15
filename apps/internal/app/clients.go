package app

import (
	"context"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/search"
)

// UserResolver resolves a GitLab username to its numeric id (TZ.md
// section 5.0). It is defined on app's own side, not gitlab's, so tests
// can stub it out without a real HTTP server (TZ.md section 2.4).
type UserResolver interface {
	ResolveUserID(ctx context.Context, username string) (int64, error)
}

// FindMRsClient is everything FindMRs needs from GitLab: the full
// search.Client surface search.Find runs on, plus username resolution.
type FindMRsClient interface {
	search.Client
	UserResolver
}

// DiscussionsClient is everything GetComments needs from GitLab: just
// the exact-count source both search strategies already share (TZ.md
// section 5.4).
type DiscussionsClient interface {
	Discussions(ctx context.Context, project gitlab.ID, mrIID int64) ([]domain.Discussion, error)
}

// The following compile-time checks prove *gitlab.Client satisfies both
// interfaces app defines for itself, without any app function signature
// ever naming the concrete type (TZ.md section 2.4).
var (
	_ FindMRsClient     = (*gitlab.Client)(nil)
	_ DiscussionsClient = (*gitlab.Client)(nil)
)
