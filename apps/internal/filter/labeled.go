package filter

import (
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// ByDate keeps only labeled_comments / filtered_comments rows whose
// CreatedAt falls inside the inclusive UTC instant range r expands to
// (TZ.md section 5.4).
func ByDate(items []artifact.LabeledCommentItem, r domain.DateRange) []artifact.LabeledCommentItem {
	out := make([]artifact.LabeledCommentItem, 0, len(items))
	for _, it := range items {
		if r.Contains(it.CreatedAt) {
			out = append(out, it)
		}
	}
	return out
}

// ByProject keeps only rows that belong to the given project id.
func ByProject(items []artifact.LabeledCommentItem, projectID int64) []artifact.LabeledCommentItem {
	out := make([]artifact.LabeledCommentItem, 0, len(items))
	for _, it := range items {
		if it.ProjectID == projectID {
			out = append(out, it)
		}
	}
	return out
}

// ByGroup keeps only rows whose project id is one of projectIDs -- the set
// of projects that belong to a group, resolved by the caller, the same
// convention MRsByGroup uses for mr_list rows.
func ByGroup(items []artifact.LabeledCommentItem, projectIDs []int64) []artifact.LabeledCommentItem {
	want := projectSet(projectIDs)
	out := make([]artifact.LabeledCommentItem, 0, len(items))
	for _, it := range items {
		if _, ok := want[it.ProjectID]; ok {
			out = append(out, it)
		}
	}
	return out
}

// ByLabel keeps only rows whose Label is one of labels -- a single
// semantic label or a whole label group (TZ.md section 4.4). Filtering by
// a label absent from the data, or absent from any taxonomy entirely,
// yields an empty result, not an error: this package only matches label
// strings, it never validates them against classify's taxonomy.
func ByLabel(items []artifact.LabeledCommentItem, labels ...string) []artifact.LabeledCommentItem {
	want := make(map[string]struct{}, len(labels))
	for _, l := range labels {
		want[l] = struct{}{}
	}
	out := make([]artifact.LabeledCommentItem, 0, len(items))
	for _, it := range items {
		if _, ok := want[it.Label]; ok {
			out = append(out, it)
		}
	}
	return out
}
