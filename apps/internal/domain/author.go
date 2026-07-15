package domain

// Author is a GitLab user as it appears on a merge request or a note:
// resolved numeric id plus username. See TZ.md section 2.3 (domain package
// type list) and section 5.0 (username-to-id resolution).
type Author struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}
