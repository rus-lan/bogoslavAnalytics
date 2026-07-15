package search

import (
	"context"
	"errors"
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

// mrLister lists one page-limited window of merge requests. It abstracts
// over Client.MergeRequests, Client.GroupMergeRequests and
// Client.ProjectMergeRequests so fetchMergeRequests can bisect a window
// without caring which scope it is listing.
type mrLister func(ctx context.Context, window gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error)

// mrListerFor picks the merge request listing call for p.Scope: a single
// project, a single group (including subgroups), or -- when neither is
// set -- every project visible to the token (TZ.md section 5.2.1). A
// project or group scope's gitlab.ID goes straight into the corresponding
// :id path parameter, whether it holds a numeric id or a namespaced path:
// no resolution step is needed here (TZ.md section 14, item 1, now
// resolved for :id path parameters).
func mrListerFor(client Client, s Scope) mrLister {
	switch {
	case s.ProjectID != nil:
		project := *s.ProjectID
		return func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return client.ProjectMergeRequests(ctx, project, w)
		}
	case s.GroupID != nil:
		group := *s.GroupID
		return func(ctx context.Context, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
			return client.GroupMergeRequests(ctx, group, w)
		}
	default:
		return client.MergeRequests
	}
}

// mrKey identifies a merge request across sub-windows: TZ.md section 6.7
// requires deduplicating by (project_id, mr_iid) after merging.
type mrKey struct {
	projectID int64
	iid       int64
}

// dedupeMergeRequests removes duplicate (project_id, iid) entries, keeping
// the first occurrence of each.
func dedupeMergeRequests(items []gitlab.MergeRequestSummary) []gitlab.MergeRequestSummary {
	seen := make(map[mrKey]bool, len(items))
	out := items[:0]
	for _, it := range items {
		k := mrKey{projectID: it.ProjectID, iid: it.IID}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, it)
	}
	return out
}

// fetchMergeRequests lists every merge request matching w via list,
// splitting w in half and recursing whenever list reports the API's
// 10,000-record page limit (gitlab.ErrPageLimitReached, TZ.md section
// 6.7).
//
// Unlike fetchEvents, splitMergeRequestWindow's two halves overlap by
// design (see its doc comment), so a merge request created before the
// split point and updated after it is returned by both halves. Every
// merge back up the recursion therefore deduplicates by (project_id,
// mr_iid) before returning, so the final result matches what an unsplit
// listing of the same window would have returned, with no duplicates and
// nothing missing.
func fetchMergeRequests(ctx context.Context, list mrLister, w gitlab.MergeRequestWindow) ([]gitlab.MergeRequestSummary, error) {
	items, err := list(ctx, w)
	if err == nil {
		return items, nil
	}
	if !errors.Is(err, gitlab.ErrPageLimitReached) {
		return nil, err
	}

	a, b, ok := splitMergeRequestWindow(w)
	if !ok {
		return items, fmt.Errorf("search: merge request window updated_after=%s created_before=%s: %w", w.UpdatedAfter, w.CreatedBefore, ErrWindowNotSplittable)
	}

	left, err := fetchMergeRequests(ctx, list, a)
	if err != nil {
		return nil, err
	}
	right, err := fetchMergeRequests(ctx, list, b)
	if err != nil {
		return nil, err
	}
	return dedupeMergeRequests(append(left, right...)), nil
}

// bruteforceCandidates builds the bruteforce-strategy candidate set
// (TZ.md section 5.2): every merge request in p.Scope, windowed by
// created_before=p.Range.To and updated_after=p.Range.From --
// updated_before is never added, since it would cut out merge requests
// updated after the window -- pre-filtered to user_notes_count >=
// p.MoreThan. That pre-filter is an upper bound on any single user's
// comment count, so it is a deliberate superset, not the final boundary:
// the exact count and the strict "> more_than" filter come later, from
// CountComments.
func bruteforceCandidates(ctx context.Context, client Client, p Params) ([]domain.MergeRequest, error) {
	list := mrListerFor(client, p.Scope)
	window := gitlab.MergeRequestWindow{CreatedBefore: p.Range.To, UpdatedAfter: p.Range.From}

	items, err := fetchMergeRequests(ctx, list, window)
	if err != nil {
		return nil, fmt.Errorf("search: bruteforce candidates: %w", err)
	}

	var out []domain.MergeRequest
	for _, it := range items {
		if it.UserNotesCount < p.MoreThan {
			continue
		}
		out = append(out, it.MergeRequest)
	}
	return out, nil
}
