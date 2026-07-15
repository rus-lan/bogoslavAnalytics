package stats

import "github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"

// FromMRList aggregates an mr_list artifact. mr_list rows are merge
// requests, not individually dated or labeled comments, so only
// SourceKind and TotalItems are filled; ByMR, ByLabel, and ByDate stay
// empty (TZ.md section 7.2.1).
func FromMRList(doc artifact.MRList) Stats {
	return Stats{
		SourceKind: artifact.KindMRList,
		TotalItems: len(doc.Items),
	}
}
