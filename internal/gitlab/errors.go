package gitlab

import "errors"

// Sentinel errors for known failure modes in the gitlab package. Failures
// that already have a domain-level sentinel (for example a username that
// does not resolve to any user) reuse domain.ErrUserNotFound instead of
// duplicating it here.
var (
	// ErrMissingToken is returned by NewClientFromEnv when GITLAB_TOKEN is
	// not set (TZ.md section 2.5).
	ErrMissingToken = errors.New("gitlab: GITLAB_TOKEN is not set")

	// ErrRateLimited is returned when a request keeps receiving 429
	// responses past the configured attempt limit.
	ErrRateLimited = errors.New("gitlab: rate limited")

	// ErrRequestTimeout is returned when a request keeps receiving 408
	// responses past the configured attempt limit (TZ.md section 5.2.6
	// and section 6.8).
	ErrRequestTimeout = errors.New("gitlab: request timeout")

	// ErrPageLimitReached is returned, together with every item collected
	// so far, when a listing consumes maxPage full pages without a short
	// page: GitLab stops reporting total counts past 10,000 records, so
	// TZ.md section 6.7 requires splitting the request into date
	// sub-windows rather than paging deeper. This package only detects
	// and reports the limit; splitting the window is done by the caller.
	ErrPageLimitReached = errors.New("gitlab: page limit reached, split the request into date sub-windows")

	// ErrProjectNotFound is returned by GetProject when GET /projects/:id
	// answers 404, so a missing project surfaces as a clear sentinel
	// instead of a zero-value domain.Project with a nil error.
	ErrProjectNotFound = errors.New("gitlab: project not found")
)
