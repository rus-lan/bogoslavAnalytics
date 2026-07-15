package stats

import "github.com/rus-lan/bogoslavAnalytics/internal/artifact"

// FromLabeledComments aggregates a labeled_comments artifact: TotalItems,
// ByMR, ByLabel, and ByDate are all filled (TZ.md section 7.2.1).
func FromLabeledComments(doc artifact.LabeledComments) Stats {
	return fromLabeledRows(artifact.KindLabeledComments, doc.Items)
}
