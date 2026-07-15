package artifact

import "github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"

// MRRef identifies a single merge request by project id and iid. It is
// used to list merge requests explicitly on a comment_list query
// (TZ.md section 4.2).
type MRRef struct {
	ProjectID int64 `json:"project_id"`
	MRIID     int64 `json:"mr_iid"`
}

// CommentQuery is the normalized query recorded on a comment_list (and,
// unchanged, a labeled_comments) artifact (TZ.md section 4.2). MRs is
// always the resolved, explicit list of merge requests the step ran
// against, whether that list was typed in directly or read off an
// mr_list artifact; FromArtifact records the latter case for
// provenance and chaining.
type CommentQuery struct {
	UserID int64       `json:"user_id"`
	From   domain.Date `json:"from"`
	To     domain.Date `json:"to"`

	MRs []MRRef `json:"mrs,omitempty"`
	// FromArtifact is the path to the mr_list artifact this query
	// chained from, if any (TZ.md section 4.2).
	FromArtifact string `json:"from_artifact,omitempty"`
}

// FilteredQuery is the normalized query recorded on a filtered_comments
// artifact (TZ.md section 4.4): it always chains from a
// labeled_comments artifact and narrows it by label, with optional
// extra date/group/project filters.
type FilteredQuery struct {
	// FromArtifact is the path to the labeled_comments artifact this
	// query filters.
	FromArtifact string   `json:"from_artifact"`
	Labels       []string `json:"labels"`

	From    *domain.Date `json:"from,omitempty"`
	To      *domain.Date `json:"to,omitempty"`
	Group   string       `json:"group,omitempty"`
	Project string       `json:"project,omitempty"`
}
