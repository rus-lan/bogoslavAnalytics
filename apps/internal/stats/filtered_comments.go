package stats

import "github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"

// FromFilteredComments aggregates a filtered_comments artifact:
// TotalItems, ByMR, ByLabel, and ByDate are all filled, identically to
// FromLabeledComments, since both kinds share the LabeledCommentItem row
// shape (TZ.md section 7.2.1).
func FromFilteredComments(doc artifact.FilteredComments) Stats {
	return fromLabeledRows(artifact.KindFilteredComments, doc.Items)
}
