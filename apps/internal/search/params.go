package search

import (
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

// Scope restricts merge request and comment-event candidates to a single
// project, to a single group's projects (including subgroups, TZ.md
// sections 5.1.6 and 5.2.1), or -- when neither is set -- to every project
// visible to the token. At most one of GroupID and ProjectID should be
// set; if both are set, ProjectID wins.
//
// GroupID and ProjectID each carry a gitlab.ID, which accepts either a
// numeric id or a namespaced path such as "my-group/my-project": GitLab
// 18.11 documents every :id path parameter this way, so a --group/--project
// path can go straight into Scope with no separate path-to-numeric-id
// resolution step (TZ.md section 14, item 1, now resolved for :id path
// parameters). The events strategy still needs a numeric project id to
// filter comment events' project_id field client-side, since that field is
// always numeric; when ProjectID was built from a namespaced path,
// scopeProjectSet resolves it via one client.GetProject call (see
// events.go).
type Scope struct {
	GroupID   *gitlab.ID
	ProjectID *gitlab.ID
}

// Params is a fully-resolved search request: Scope's ids are either already
// numeric or a namespaced path gitlab/'s :id parameters accept directly.
// Resolving --user (username or id, TZ.md section 5.0) to a numeric id, and
// routing the point mode of TZ.md section 7.2 (an explicit project+mr with
// no candidate search at all), remain the caller's job, not search's.
type Params struct {
	UserID   int64
	Range    domain.DateRange
	MoreThan int
	Scope    Scope
}
