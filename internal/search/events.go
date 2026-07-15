package search

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// noteableTypeMergeRequest is the note.noteable_type value that marks a
// comment event as belonging to a merge request (TZ.md section 5.1.3).
const noteableTypeMergeRequest = "MergeRequest"

// fetchEvents fetches every comment event for userID across r, splitting
// the range in half and recursing whenever the client reports the API's
// 10,000-record page limit (gitlab.ErrPageLimitReached, TZ.md section
// 6.7). Every event carries a single created_at timestamp, so this split
// is a clean partition of r: no duplicates and no gaps, so no
// deduplication step is needed here (unlike the merge request list, see
// bruteforce.go).
func fetchEvents(ctx context.Context, client Client, userID int64, r domain.DateRange) ([]gitlab.CommentEvent, error) {
	events, err := client.CommentEvents(ctx, userID, r)
	if err == nil {
		return events, nil
	}
	if !errors.Is(err, gitlab.ErrPageLimitReached) {
		return nil, err
	}

	left, right, ok := splitDateRange(r)
	if !ok {
		return events, fmt.Errorf("search: events window %s..%s: %w", r.From, r.To, ErrWindowNotSplittable)
	}

	leftEvents, err := fetchEvents(ctx, client, userID, left)
	if err != nil {
		return nil, err
	}
	rightEvents, err := fetchEvents(ctx, client, userID, right)
	if err != nil {
		return nil, err
	}
	return append(leftEvents, rightEvents...), nil
}

// scopeProjectNumericID returns the numeric project id p represents, when p
// was built as a numeric gitlab.ID (gitlab.NumericID). ok is false when p is
// a namespaced path: gitlab.ID.String() documents returning "the plain
// (unescaped) path" for a path id, and a valid GitLab project path always
// carries at least one non-digit character (the namespace separator),
// which strconv.ParseInt correctly rejects.
func scopeProjectNumericID(p gitlab.ID) (int64, bool) {
	n, err := strconv.ParseInt(p.String(), 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// scopeProjectSet resolves p.Scope into the set of project ids the events
// strategy must keep. It returns nil for "no restriction" (every project
// visible to the token), matching TZ.md section 5.1.6's client-side group
// filter ("события чужих проектов отбрасываются на клиенте").
//
// A group scope never needs a resolution step: client.GroupProjects always
// returns numeric project ids (TZ.md section 5.1.6), whether the group
// itself was given as a numeric id or a namespaced path. A project scope
// does need one when it was built from a namespaced path, since comment
// events' project_id field (TZ.md section 5.1.2) is always numeric: that
// one lookup goes through client.GetProject, exactly once per call here
// (never once per candidate event).
func scopeProjectSet(ctx context.Context, client Client, s Scope) (map[int64]bool, error) {
	switch {
	case s.ProjectID != nil:
		id, ok := scopeProjectNumericID(*s.ProjectID)
		if !ok {
			project, err := client.GetProject(ctx, *s.ProjectID)
			if err != nil {
				return nil, fmt.Errorf("search: scope project %s: %w", s.ProjectID.String(), err)
			}
			id = project.ID
		}
		return map[int64]bool{id: true}, nil
	case s.GroupID != nil:
		projects, err := client.GroupProjects(ctx, *s.GroupID)
		if err != nil {
			return nil, fmt.Errorf("search: scope group %s projects: %w", s.GroupID.String(), err)
		}
		set := make(map[int64]bool, len(projects))
		for _, pr := range projects {
			set[pr.ID] = true
		}
		return set, nil
	default:
		return nil, nil
	}
}

// eventsCandidates builds the events-strategy candidate set (TZ.md section
// 5.1): fetch p.UserID's comment events over p.Range, map each one onto a
// (project_id, note.noteable_iid) pair, keep only events whose
// note.noteable_type is "MergeRequest" and whose note.system is false
// (dropping every other event -- including plain non-thread notes on
// issues, commits, etc. and system notes -- on the client, per TZ.md
// section 5.1.3), restrict to p.Scope, and keep only merge requests whose
// preliminary count (one event = one comment, TZ.md section 5.1.2) is at
// least p.MoreThan. That preliminary count is a deliberate superset, not
// the final boundary: the exact count and the strict "> more_than" filter
// come later, from CountComments.
func eventsCandidates(ctx context.Context, client Client, p Params) ([]domain.MergeRequest, error) {
	events, err := fetchEvents(ctx, client, p.UserID, p.Range)
	if err != nil {
		return nil, fmt.Errorf("search: events candidates: %w", err)
	}

	projects, err := scopeProjectSet(ctx, client, p.Scope)
	if err != nil {
		return nil, err
	}

	type key struct {
		projectID int64
		mrIID     int64
	}
	counts := make(map[key]int)
	var order []key
	for _, e := range events {
		if e.Note.NoteableType != noteableTypeMergeRequest || e.Note.System {
			continue
		}
		if projects != nil && !projects[e.ProjectID] {
			continue
		}
		k := key{projectID: e.ProjectID, mrIID: e.Note.NoteableIID}
		if _, seen := counts[k]; !seen {
			order = append(order, k)
		}
		counts[k]++
	}

	var out []domain.MergeRequest
	for _, k := range order {
		if counts[k] < p.MoreThan {
			continue
		}
		out = append(out, domain.MergeRequest{ProjectID: k.projectID, IID: k.mrIID})
	}
	return out, nil
}
