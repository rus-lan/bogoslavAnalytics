package domain

import (
	"strings"
	"time"
)

// MergeRequest is a GitLab merge request together with the per-user
// comment count computed for it by find_mrs (TZ.md section 4.1).
//
// ProjectPath is the namespaced project path (e.g. "my-group/my-project").
// It has no producer in this package: nothing here fills it in from
// References, that is left to a caller (gitlab/) that chooses to derive
// it via References.ProjectPath.
type MergeRequest struct {
	ProjectID    int64      `json:"project_id"`
	ProjectPath  string     `json:"project_path,omitempty"`
	IID          int64      `json:"iid"`
	Title        string     `json:"title"`
	WebURL       string     `json:"web_url"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Author       Author     `json:"author"`
	CommentCount int        `json:"comment_count"`
	References   References `json:"references"`
}

// References mirrors the "references" object GitLab attaches to a merge
// request in list responses: short, relative and full forms of a
// reference to it, e.g. {"short": "!1", "relative": "!1", "full":
// "my-group/my-project!1"}.
type References struct {
	Short    string `json:"short"`
	Relative string `json:"relative"`
	Full     string `json:"full"`
}

// ProjectPath returns the namespaced project path carried in r.Full, e.g.
// "my-group/my-project" out of "my-group/my-project!123".
//
// GitLab documents Full as "Complete reference to a merge request,
// including full project path, like gitlab-org/gitlab!123", i.e. the
// format "<full project path>!<iid>". A GitLab project path cannot
// itself contain "!", so splitting on the LAST "!" recovers the project
// path regardless of how many digits the iid has. This split rule is
// derived from that documented format; the split rule itself is not
// part of the documented API contract.
//
// ProjectPath returns "" if r.Full is empty or has no "!".
func (r References) ProjectPath() string {
	i := strings.LastIndex(r.Full, "!")
	if i < 0 {
		return ""
	}
	return r.Full[:i]
}
