package artifact

import (
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

// MRItem is a single mr_list row: a merge request together with the
// exact comment count found for it (TZ.md section 4.1). ProjectPath,
// Title, WebURL, CreatedAt and UpdatedAt are all optional. ProjectPath
// has no producer yet: no converter from domain.MergeRequest or
// search.Result can fill it in today (there is no path field on
// domain.MergeRequest), so a writer that does not have it simply
// leaves it zero-valued and it is omitted on the wire, same as the
// other display-only fields added for the html format (write-only,
// presentation only), which can link a merge request's title to its
// web_url and show when it was created and last updated, the same
// information domain.MergeRequest already carries.
type MRItem struct {
	ProjectID    int64  `json:"project_id"`
	ProjectPath  string `json:"project_path,omitempty"`
	MRIID        int64  `json:"mr_iid"`
	CommentCount int    `json:"comment_count"`

	Title     string    `json:"title,omitempty"`
	WebURL    string    `json:"web_url,omitempty"`
	CreatedAt time.Time `json:"created_at,omitzero"`
	UpdatedAt time.Time `json:"updated_at,omitzero"`
}

// CommentItem is a single comment_list row: a note together with the
// iid of the merge request it belongs to (TZ.md section 4.2). The iid
// is carried alongside the note rather than folded into it because the
// note's own NoteableID is the merge request's internal id, not its
// iid.
type CommentItem struct {
	MRIID int64 `json:"mr_iid"`
	domain.Note
}

// LabeledCommentItem is a single labeled_comments row: a CommentItem
// carrying the semantic label assigned to it by the calling agent
// (TZ.md section 4.3). It is also the row shape of a filtered_comments
// artifact, which holds a label-filtered subset of the same rows
// (TZ.md section 4.4).
type LabeledCommentItem struct {
	MRIID int64 `json:"mr_iid"`
	domain.LabeledNote
}

// Taxonomy is the copy of the applied label taxonomy recorded on a
// labeled_comments artifact (TZ.md section 4.3).
type Taxonomy struct {
	Version int      `json:"version"`
	Labels  []string `json:"labels"`
}
