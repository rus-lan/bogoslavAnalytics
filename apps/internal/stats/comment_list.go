package stats

import "github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"

// FromCommentList aggregates a comment_list artifact: TotalItems, ByMR,
// and ByDate are filled. ByLabel stays empty, since comment_list rows
// carry no label (TZ.md section 7.2.1).
func FromCommentList(doc artifact.CommentList) Stats {
	byMR := make(map[mrKey]int, len(doc.Items))
	byDate := make(map[string]int, len(doc.Items))
	for _, it := range doc.Items {
		byMR[mrKey{it.ProjectID, it.MRIID}]++
		byDate[dateBucket(it.CreatedAt)]++
	}
	return Stats{
		SourceKind: artifact.KindCommentList,
		TotalItems: len(doc.Items),
		ByMR:       sortedMRCounts(byMR),
		ByDate:     byDate,
	}
}
