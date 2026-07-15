package search

import "github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"

// Result is the outcome of Find: the strategy actually used, the smoke
// test result behind that choice, and the merge requests found, each
// already carrying its exact comment count and already filtered to the
// comment_count > MoreThan boundary (TZ.md section 4.1).
//
// Smoke is the zero value (empty string) when the smoke test never ran --
// SelectStrategy skips it whenever strict mode or the retention-window
// check alone already forces bruteforce (TZ.md section 5.3).
type Result struct {
	Strategy domain.Strategy
	Smoke    domain.SmokeResult
	Items    []domain.MergeRequest
}
