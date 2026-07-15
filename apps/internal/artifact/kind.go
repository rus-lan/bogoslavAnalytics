package artifact

// Kind names the four artifact shapes the pipeline writes (TZ.md section 4).
type Kind string

const (
	// KindMRList is artifact-1: merge requests found by find_mrs, with
	// exact comment counts (TZ.md section 4.1).
	KindMRList Kind = "mr_list"
	// KindCommentList is artifact-2: comments pulled for a user across a
	// set of merge requests (TZ.md section 4.2).
	KindCommentList Kind = "comment_list"
	// KindLabeledComments is artifact-3: comment_list plus semantic
	// labels and classifier provenance (TZ.md section 4.3).
	KindLabeledComments Kind = "labeled_comments"
	// KindFilteredComments is artifact-4: a label-filtered subset of a
	// labeled_comments artifact (TZ.md section 4.4).
	KindFilteredComments Kind = "filtered_comments"
)
