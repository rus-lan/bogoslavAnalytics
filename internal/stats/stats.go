package stats

import "github.com/rus-lan/bogoslavAnalytics/internal/artifact"

// MRCount is one row of a by_mr breakdown: how many rows of the input
// artifact belong to one merge request (TZ.md section 7.2.1).
type MRCount struct {
	ProjectID int64 `json:"project_id"`
	MRIID     int64 `json:"mr_iid"`
	Count     int   `json:"count"`
}

// Stats is the get_stats output contract (TZ.md section 7.2.1): aggregates
// over the items of one already-decoded input artifact, of any kind. It
// carries no schema_version/source/query block, because get_stats does not
// write a new cached artifact -- it returns a plain summary of an existing
// one.
type Stats struct {
	// SourceKind is the kind of the input artifact.
	SourceKind artifact.Kind `json:"source_kind"`
	// TotalItems is the number of rows in the input artifact's items.
	TotalItems int `json:"total_items"`

	// ByMR breaks TotalItems down by merge request, sorted by
	// (project_id, mr_iid) for a stable, reproducible order rather than
	// depending on map iteration order. It is filled for comment_list,
	// labeled_comments, and filtered_comments inputs; mr_list rows
	// already are merge requests, so there is no separate row to bucket
	// by MR (TZ.md section 7.2.1).
	ByMR []MRCount `json:"by_mr,omitempty"`

	// ByLabel breaks TotalItems down by semantic label. It is filled
	// only for labeled_comments and filtered_comments inputs, the two
	// kinds whose rows carry a label.
	ByLabel map[string]int `json:"by_label,omitempty"`

	// ByDate breaks TotalItems down by the UTC calendar day
	// (YYYY-MM-DD) of each row's CreatedAt, the same UTC day convention
	// domain.DateRange uses (TZ.md section 5.4). It is filled for the
	// same three kinds as ByMR.
	ByDate map[string]int `json:"by_date,omitempty"`
}
