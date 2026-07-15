package stats

import (
	"sort"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
)

// mrKey identifies one merge request for the by_mr breakdown.
type mrKey struct {
	projectID int64
	mrIID     int64
}

// dateBucket renders t's UTC calendar day as "YYYY-MM-DD", the by_date
// bucket key (TZ.md section 7.2.1).
func dateBucket(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// sortedMRCounts turns a project+mr count map into a slice sorted by
// (project_id, mr_iid), the stable order Stats.ByMR reports in.
func sortedMRCounts(counts map[mrKey]int) []MRCount {
	out := make([]MRCount, 0, len(counts))
	for k, count := range counts {
		out = append(out, MRCount{ProjectID: k.projectID, MRIID: k.mrIID, Count: count})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProjectID != out[j].ProjectID {
			return out[i].ProjectID < out[j].ProjectID
		}
		return out[i].MRIID < out[j].MRIID
	})
	return out
}

// fromLabeledRows aggregates the row shape shared by labeled_comments and
// filtered_comments artifacts: every field of Stats is filled.
func fromLabeledRows(kind artifact.Kind, items []artifact.LabeledCommentItem) Stats {
	byMR := make(map[mrKey]int, len(items))
	byLabel := make(map[string]int, len(items))
	byDate := make(map[string]int, len(items))
	for _, it := range items {
		byMR[mrKey{it.ProjectID, it.MRIID}]++
		byLabel[it.Label]++
		byDate[dateBucket(it.CreatedAt)]++
	}
	return Stats{
		SourceKind: kind,
		TotalItems: len(items),
		ByMR:       sortedMRCounts(byMR),
		ByLabel:    byLabel,
		ByDate:     byDate,
	}
}
