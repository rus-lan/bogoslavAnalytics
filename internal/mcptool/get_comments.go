package mcptool

import "github.com/rus-lan/bogoslavAnalytics/internal/artifact"

// GetCommentsInput is the get_comments tool's input: the MCP mirror of
// bogoslav-cli's get-comments command and app.GetCommentsRequest (TZ.md
// sections 7.2, 7.3).
type GetCommentsInput struct {
	User         string           `json:"user" jsonschema:"GitLab username or numeric user id"`
	From         string           `json:"from" jsonschema:"start of the date range, inclusive, YYYY-MM-DD"`
	To           string           `json:"to" jsonschema:"end of the date range, inclusive, YYYY-MM-DD"`
	FromArtifact string           `json:"from_artifact,omitempty" jsonschema:"path to an existing mr_list artifact (json or yaml only) whose merge requests to fetch comments for; mutually exclusive with mrs"`
	MRs          []artifact.MRRef `json:"mrs,omitempty" jsonschema:"explicit merge request list (project_id, mr_iid pairs) to fetch comments for; mutually exclusive with from_artifact"`

	ArtifactsDir    string `json:"artifacts_dir,omitempty" jsonschema:"directory the comment_list artifact is written under, and where a matching artifact is looked up as a cache before calling GitLab (default \"artifacts\")"`
	Format          string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml). Artifacts double as the cache: json and yaml round-trip and are looked up before calling GitLab. text and html are write-only -- neither is readable back, neither can ever be chained via from_artifact into a later tool, and neither is ever a cache hit"`
	Refresh         bool   `json:"refresh,omitempty" jsonschema:"bypass the cache and always call GitLab, even if a fresh cached comment_list artifact already exists"`
	CacheTTLSeconds int64  `json:"cache_ttl_seconds,omitempty" jsonschema:"how long, in seconds, a cached comment_list artifact stays fresh before this tool calls GitLab again (default 86400, 24h)"`
}

// GetCommentsOutput is the get_comments tool's output.
type GetCommentsOutput struct {
	Path     string `json:"path" jsonschema:"path to the written (or, on a cache hit, already-existing) comment_list artifact"`
	CacheHit bool   `json:"cache_hit" jsonschema:"true when this result came from an existing artifact without calling GitLab"`
	Count    int    `json:"count" jsonschema:"number of comments in the result"`
}
