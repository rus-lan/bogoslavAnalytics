package domain

// Strategy is the MR search strategy actually used to build an mr_list
// artifact (TZ.md section 5).
type Strategy string

const (
	// StrategyEvents is the primary strategy: candidates come from
	// GET /users/:id/events, pinned by an exact /discussions count.
	StrategyEvents Strategy = "events"
	// StrategyBruteforce is the fallback strategy: candidates come from
	// listing every MR in scope.
	StrategyBruteforce Strategy = "bruteforce"
)

// SmokeResult is the outcome of the DiscussionNote smoke test that gates
// the strategy autoselector (TZ.md section 5.5).
type SmokeResult string

const (
	SmokePassed  SmokeResult = "passed"
	SmokeFailed  SmokeResult = "failed"
	SmokeUnknown SmokeResult = "unknown"
)

// Query is the normalized request that drives a pipeline step and is
// hashed to build the artifact cache key (TZ.md sections 4.1 and 4.5).
// It carries the resolved GitLab connection, the resolved numeric user
// id, and every parameter that changes the result set.
type Query struct {
	// GitlabURL and UserID are required in the canonical, hashed form
	// of the query even though the illustrative artifact YAML in
	// TZ.md renders GitlabURL under the separate "source" block
	// (TZ.md section 4.5).
	GitlabURL string `json:"gitlab_url"`
	UserID    int64  `json:"user_id"`

	From Date `json:"from"`
	To   Date `json:"to"`

	// MoreThan is the N threshold. The final predicate is strictly
	// comment_count > MoreThan (TZ.md section 4.1); MoreThan itself is
	// the raw request parameter, not the predicate.
	MoreThan int `json:"more_than"`

	Group   string `json:"group,omitempty"`
	Project string `json:"project,omitempty"`
	// MR is the merge request iid for point mode: set together with
	// Project to target a single MR without running candidate search
	// (TZ.md section 7.2).
	MR *int64 `json:"mr,omitempty"`

	Strategy Strategy    `json:"strategy,omitempty"`
	Smoke    SmokeResult `json:"smoke,omitempty"`
}
