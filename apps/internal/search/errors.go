package search

import "errors"

// Sentinel errors for known failure modes in the search package.
var (
	// ErrWindowNotSplittable is returned when a listing endpoint reports
	// the 10,000-record page limit (gitlab.ErrPageLimitReached) but the
	// date window driving it has already shrunk to a single day and still
	// overflows: TZ.md section 6.7's sub-window split has nothing left to
	// cut in half.
	ErrWindowNotSplittable = errors.New("search: window cannot be split further")

	// ErrUnknownStrategy guards the Find switch over the strategy
	// SelectStrategy returns. It should never surface in practice, since
	// SelectStrategy only ever returns domain.StrategyEvents or
	// domain.StrategyBruteforce.
	ErrUnknownStrategy = errors.New("search: unknown strategy")
)
