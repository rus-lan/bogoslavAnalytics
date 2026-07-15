package search

import (
	"context"
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// enrichEventsCandidates fills in Title, WebURL, CreatedAt, UpdatedAt and
// ProjectPath for the events strategy's survivors -- the candidates that
// already passed the exact "> MoreThan" filter (TZ.md section 4.1) -- so
// both strategies produce the same domain.MergeRequest shape for the same
// merge request (TZ.md section 14, item 11: eventsCandidates only ever
// builds a ProjectID/IID shell, unlike bruteforceCandidates, which already
// carries the full shape straight from the list endpoint).
//
// Only survivors are enriched here, never every candidate: the whole point
// of the events strategy is that its candidate set stays cheap, and
// survivors are a small fraction of it. Enrichment issues exactly one
// ProjectMergeRequestsByIIDs call per distinct project among the survivors
// -- iids[] is documented only on the project-scoped merge request list
// endpoint (gitlab.ProjectMergeRequestsByIIDs), so batching per project is
// the only shape this call can take; it is never one call per survivor and
// never one call across the whole set.
//
// A survivor whose enrichment fetch has no matching entry (for example, the
// merge request was deleted after the comment was made) keeps its existing
// ProjectID/IID/CommentCount rather than being dropped from the result.
func enrichEventsCandidates(ctx context.Context, client Client, items []domain.MergeRequest) ([]domain.MergeRequest, error) {
	if len(items) == 0 {
		return items, nil
	}

	var order []int64
	iidsByProject := make(map[int64][]int64)
	for _, it := range items {
		if _, seen := iidsByProject[it.ProjectID]; !seen {
			order = append(order, it.ProjectID)
		}
		iidsByProject[it.ProjectID] = append(iidsByProject[it.ProjectID], it.IID)
	}

	found := make(map[mrKey]gitlab.MergeRequestSummary, len(items))
	for _, projectID := range order {
		summaries, err := client.ProjectMergeRequestsByIIDs(ctx, gitlab.NumericID(projectID), iidsByProject[projectID])
		if err != nil {
			return nil, fmt.Errorf("search: enrich events candidates for project %d: %w", projectID, err)
		}
		for _, s := range summaries {
			found[mrKey{projectID: s.ProjectID, iid: s.IID}] = s
		}
	}

	out := make([]domain.MergeRequest, len(items))
	for i, it := range items {
		s, ok := found[mrKey{projectID: it.ProjectID, iid: it.IID}]
		if !ok {
			out[i] = it
			continue
		}
		mr := s.MergeRequest
		mr.CommentCount = it.CommentCount
		out[i] = mr
	}
	return out, nil
}
