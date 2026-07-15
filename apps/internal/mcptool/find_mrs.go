package mcptool

// FindMRsInput is the find_mrs tool's input: the MCP mirror of
// bogoslav-cli's find-mrs command and app.FindMRsRequest (TZ.md sections
// 7.2, 7.3).
type FindMRsInput struct {
	User string `json:"user" jsonschema:"GitLab username or numeric user id"`
	From string `json:"from" jsonschema:"start of the date range, inclusive, YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"end of the date range, inclusive, YYYY-MM-DD"`
	// MoreThan is the N threshold: the boundary is honestly STRICT, not
	// >=. See the field's own description for the exact contract.
	MoreThan int    `json:"more_than" jsonschema:"return only merge requests where user left STRICTLY more than this many comments in [from, to]; a merge request with exactly this many comments is NOT returned, only one with more_than + 1 or more"`
	Group    string `json:"group,omitempty" jsonschema:"restrict the search to this group's projects, including subgroups (numeric id or path)"`
	Project  string `json:"project,omitempty" jsonschema:"restrict the search to this single project (numeric id or path); required together with mr"`
	MR       int64  `json:"mr,omitempty" jsonschema:"point mode: return exactly this merge request iid, with no candidate search of any kind (no events, no bruteforce, no autoselector); requires project; 0 or omitted means no point mode"`
	Strict   bool   `json:"strict,omitempty" jsonschema:"force the bruteforce search strategy, skipping the events strategy and its smoke test"`

	ArtifactsDir    string `json:"artifacts_dir,omitempty" jsonschema:"directory the mr_list artifact is written under, and where a matching artifact is looked up as a cache before calling GitLab (default \"artifacts\")"`
	Format          string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml). Artifacts double as the cache: json and yaml round-trip and are looked up before calling GitLab. text and html are write-only -- neither is readable back, neither can ever be chained via from_artifact into a later tool, and neither is ever a cache hit"`
	Refresh         bool   `json:"refresh,omitempty" jsonschema:"bypass the cache and always call GitLab, even if a fresh cached mr_list artifact already exists"`
	CacheTTLSeconds int64  `json:"cache_ttl_seconds,omitempty" jsonschema:"how long, in seconds, a cached mr_list artifact stays fresh before this tool calls GitLab again (default 86400, 24h)"`
}
