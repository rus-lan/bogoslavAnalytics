package mcptool

// FilterCommentsInput is the filter_comments tool's input: the MCP
// mirror of bogoslav-cli's filter-comments command and
// app.FilterCommentsRequest (TZ.md sections 7.2, 7.3).
type FilterCommentsInput struct {
	FromArtifact string   `json:"from_artifact" jsonschema:"path to the labeled_comments artifact (json or yaml only) to filter"`
	Labels       []string `json:"labels" jsonschema:"taxonomy labels to keep; at least one is required"`
	From         string   `json:"from,omitempty" jsonschema:"start of an additional date filter, inclusive, YYYY-MM-DD; requires to"`
	To           string   `json:"to,omitempty" jsonschema:"end of an additional date filter, inclusive, YYYY-MM-DD; requires from"`
	Group        string   `json:"group,omitempty" jsonschema:"keep only comments on merge requests in this group's projects, including subgroups (numeric id or path); resolved to project ids with one GitLab call"`
	Project      string   `json:"project,omitempty" jsonschema:"keep only comments on merge requests in this single project (numeric id or path); resolved with one GitLab call if it is a namespaced path"`

	ArtifactsDir string `json:"artifacts_dir,omitempty" jsonschema:"directory the filtered_comments artifact is written under (default \"artifacts\")"`
	Format       string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml); this tool never consults a cache before running -- it always reads from_artifact and reprocesses it"`
}
