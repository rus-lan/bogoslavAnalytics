package app

import "errors"

// Sentinel errors for known failure modes in the app use-case layer.
var (
	// ErrPointModeRequiresProject is returned by FindMRs when MR is set
	// without Project: point mode (TZ.md sections 1.2, 7.2) always names
	// a single (project, mr) pair.
	ErrPointModeRequiresProject = errors.New("app: point mode requires a project together with mr")

	// ErrMergeRequestNotFound is returned by FindMRs' point mode when the
	// requested project+mr does not resolve to any merge request.
	ErrMergeRequestNotFound = errors.New("app: merge request not found")

	// ErrNoMergeRequests is returned by GetComments when neither
	// FromArtifact nor an explicit MRs list names any merge request to
	// fetch comments for.
	ErrNoMergeRequests = errors.New("app: no merge requests to fetch comments for")

	// ErrAmbiguousMergeRequests is returned by GetComments when both
	// FromArtifact and an explicit MRs list are set: exactly one must
	// name the merge request set.
	ErrAmbiguousMergeRequests = errors.New("app: set either from_artifact or an explicit merge request list, not both")

	// ErrNoLabels is returned by FilterComments when Labels is empty:
	// filtering by zero labels would silently keep nothing, which is
	// never what a caller means.
	ErrNoLabels = errors.New("app: filter comments requires at least one label")

	// ErrIncompleteDateFilter is returned by FilterComments when only
	// one of From/To is set: a date range filter needs both ends.
	ErrIncompleteDateFilter = errors.New("app: filter comments date filter requires both from and to")
)
