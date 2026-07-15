package domain

// Project is a GitLab project (repository): numeric id plus its
// namespaced path (e.g. "my-group/repo").
type Project struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}
