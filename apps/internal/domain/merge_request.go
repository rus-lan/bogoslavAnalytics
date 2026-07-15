package domain

import "time"

// MergeRequest is a GitLab merge request together with the per-user
// comment count computed for it by find_mrs (TZ.md section 4.1).
type MergeRequest struct {
	ProjectID    int64     `json:"project_id"`
	IID          int64     `json:"iid"`
	Title        string    `json:"title"`
	WebURL       string    `json:"web_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Author       Author    `json:"author"`
	CommentCount int       `json:"comment_count"`
}
