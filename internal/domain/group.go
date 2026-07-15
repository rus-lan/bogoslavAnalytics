package domain

// Group is a GitLab group: numeric id plus its path (e.g. "my-group").
type Group struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}
