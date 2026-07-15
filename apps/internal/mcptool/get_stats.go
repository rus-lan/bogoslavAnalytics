package mcptool

// GetStatsInput is the get_stats tool's input: the MCP mirror of
// bogoslav-cli's get-stats command and app.GetStatsRequest (TZ.md
// sections 7.2, 7.2.1, 7.3). get_stats never calls GitLab: it only
// aggregates an already-written artifact's items.
type GetStatsInput struct {
	ArtifactPath string `json:"artifact_path" jsonschema:"path to any one of the four artifact kinds (mr_list, comment_list, labeled_comments, filtered_comments) to aggregate"`
	ArtifactsDir string `json:"artifacts_dir,omitempty" jsonschema:"when set, also writes the aggregate as a stats_<name>.<ext> file (json or yaml only) under this directory; omit to only return the aggregate without writing a file"`
	Format       string `json:"format,omitempty" jsonschema:"output format for the written stats file when artifacts_dir is set: json or yaml only (default yaml); stats is not one of the four artifact kinds and has no text or html rendering of its own"`
}
