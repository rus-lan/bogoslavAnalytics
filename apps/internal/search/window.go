package search

import (
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// addDays returns the date n days after d.
func addDays(d domain.Date, n int) domain.Date {
	t := d.Start().AddDate(0, 0, n)
	return domain.NewDate(t.Year(), t.Month(), t.Day())
}

// midDate returns a date strictly between from and to, given from is
// strictly before to. Both ranges the caller builds from it (from..mid and
// something after mid..to) are therefore proper, non-empty subsets of
// from..to, so repeated splitting always terminates at single-day ranges.
func midDate(from, to domain.Date) domain.Date {
	days := int(to.Start().Sub(from.Start()).Hours() / 24)
	t := from.Start().AddDate(0, 0, days/2)
	return domain.NewDate(t.Year(), t.Month(), t.Day())
}

// splitDateRange splits r into two non-overlapping halves: [r.From, mid]
// and [mid+1 day, r.To]. This is a clean partition -- no date falls in
// both halves and none is skipped -- which is why it is safe to use for
// events, where every record carries a single created_at timestamp (TZ.md
// section 6.7, last bullet). ok is false when r already spans a single
// day and so cannot be split further.
func splitDateRange(r domain.DateRange) (left, right domain.DateRange, ok bool) {
	if !r.From.Before(r.To) {
		return domain.DateRange{}, domain.DateRange{}, false
	}
	mid := midDate(r.From, r.To)
	left, _ = domain.NewDateRange(r.From, mid)
	right, _ = domain.NewDateRange(addDays(mid, 1), r.To)
	return left, right, true
}

// splitMergeRequestWindow splits the bruteforce window predicate
// (created_before=w.CreatedBefore, updated_after=w.UpdatedAfter) into two
// sub-windows around a midpoint date, exactly as TZ.md section 6.7
// prescribes: window a keeps the original UpdatedAfter but caps
// CreatedBefore at mid; window b keeps the original CreatedBefore but
// raises UpdatedAfter to mid.
//
// Unlike splitDateRange, this is NOT a clean partition: the predicate is
// asymmetric (created_before + updated_after, never updated_before), so a
// merge request created before mid and updated after mid matches both a
// and b. That overlap is intentional -- it is what keeps a merge request
// updated exactly at mid from falling into neither window -- and it is why
// callers must deduplicate the merged results by (project_id, mr_iid).
//
// ok is false when w already spans a single day and so cannot be split
// further.
func splitMergeRequestWindow(w gitlab.MergeRequestWindow) (a, b gitlab.MergeRequestWindow, ok bool) {
	from, to := w.UpdatedAfter, w.CreatedBefore
	if !from.Before(to) {
		return gitlab.MergeRequestWindow{}, gitlab.MergeRequestWindow{}, false
	}
	mid := midDate(from, to)
	a = gitlab.MergeRequestWindow{CreatedBefore: mid, UpdatedAfter: from}
	b = gitlab.MergeRequestWindow{CreatedBefore: to, UpdatedAfter: mid}
	return a, b, true
}
