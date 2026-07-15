package search

import "github.com/rus-lan/bogoslav-analytics/apps/internal/domain"

// Scope restricts merge request and comment-event candidates to a single
// project, to a single group's projects (including subgroups, TZ.md
// sections 5.1.6 and 5.2.1), or -- when neither is set -- to every project
// visible to the token. At most one of GroupID and ProjectID should be
// set; if both are set, ProjectID wins.
type Scope struct {
	GroupID   *int64
	ProjectID *int64
}

// Params is a fully-resolved search request. Every id is already numeric:
// resolving --user (username or id, TZ.md section 5.0) and a --group or
// --project path to a numeric id, as well as routing the point mode of
// TZ.md section 7.2 (an explicit project+mr with no candidate search at
// all), are the caller's job, not search's.
type Params struct {
	UserID   int64
	Range    domain.DateRange
	MoreThan int
	Scope    Scope
}
