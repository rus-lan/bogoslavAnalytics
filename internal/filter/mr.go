package filter

import "github.com/rus-lan/bogoslavAnalytics/internal/artifact"

// MRsByCount keeps only mr_list rows whose comment count is strictly
// greater than moreThan. This is the final threshold predicate pinned by
// TZ.md section 4.1: comment_count > more_than, never >=. Candidate
// pre-filters elsewhere in the pipeline (events, bruteforce) use >=/< as a
// superset estimate over candidates; this function is what pins the exact
// boundary.
func MRsByCount(items []artifact.MRItem, moreThan int) []artifact.MRItem {
	out := make([]artifact.MRItem, 0, len(items))
	for _, it := range items {
		if it.CommentCount > moreThan {
			out = append(out, it)
		}
	}
	return out
}

// MRsByProject keeps only mr_list rows that belong to the given project id.
func MRsByProject(items []artifact.MRItem, projectID int64) []artifact.MRItem {
	out := make([]artifact.MRItem, 0, len(items))
	for _, it := range items {
		if it.ProjectID == projectID {
			out = append(out, it)
		}
	}
	return out
}

// MRsByGroup keeps only mr_list rows whose project id is one of
// projectIDs -- the set of projects that belong to a group, including
// subgroups (GET /groups/:id/projects?include_subgroups=true), resolved
// by the caller before filtering runs. This package makes no HTTP calls
// and has no notion of GitLab groups beyond the id set it is given.
func MRsByGroup(items []artifact.MRItem, projectIDs []int64) []artifact.MRItem {
	want := projectSet(projectIDs)
	out := make([]artifact.MRItem, 0, len(items))
	for _, it := range items {
		if _, ok := want[it.ProjectID]; ok {
			out = append(out, it)
		}
	}
	return out
}

// MRPoint keeps the single mr_list row matching one project and merge
// request iid -- point mode (TZ.md section 7.2): find_mrs called with
// project+mr returns exactly this one row, without candidate search.
func MRPoint(items []artifact.MRItem, projectID, mrIID int64) []artifact.MRItem {
	out := make([]artifact.MRItem, 0, 1)
	for _, it := range items {
		if it.ProjectID == projectID && it.MRIID == mrIID {
			out = append(out, it)
		}
	}
	return out
}

// projectSet builds a lookup set of project ids, shared by the group
// filters over both mr_list rows and labeled_comments rows.
func projectSet(projectIDs []int64) map[int64]struct{} {
	set := make(map[int64]struct{}, len(projectIDs))
	for _, id := range projectIDs {
		set[id] = struct{}{}
	}
	return set
}
