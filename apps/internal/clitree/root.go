package clitree

import "github.com/spf13/cobra"

// NewRootCmd builds the bogoslav-cli command tree: exactly six commands,
// one per apps/internal/app use case (TZ.md section 7.3). bogoslav-skills
// (a later wave) generates SKILL.md by walking this tree, so command
// names, flag names and help text are user-facing product surface, not
// decoration.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bogoslav-cli",
		Short: "GitLab review-activity analytics: find, fetch, label, filter, and summarize merge request comments",
		Long: `bogoslav-cli answers one question: in which merge requests did a
GitLab user leave more than N comments -- with filters by date, group and
repository -- and carries that answer through a pipeline of further steps.

Each command below is a thin wrapper over exactly one function in
apps/internal/app (TZ.md section 7.3): it parses flags, builds a request,
calls that function, and renders the result. bogoslav-mcp exposes the exact
same six operations as MCP tools over the same functions.

Pipeline: find-mrs -> get-comments -> get-classify-batch -> save-labels ->
filter-comments -> get-stats. Any step also runs on its own: get-comments,
get-classify-batch, filter-comments and get-stats all accept --from-artifact
to chain from a previous step's output file instead of running the whole
pipeline.

Artifacts and cache: find-mrs, get-comments, filter-comments and
save-labels each write their result under --artifacts-dir (default
"./artifacts") in --format (json, yaml, text, or html). For find-mrs and
get-comments, that same file also doubles as the cache for the next
identical request: json and yaml artifacts are looked up, by their
normalized request, before either calls GitLab; --refresh bypasses the
cache and always calls GitLab. text and html are write-only: neither
round-trips back into structured data, so neither is ever a cache hit and
neither can be passed to --from-artifact.

get-classify-batch also looks --artifacts-dir up as a cache -- a matching
labeled_comments artifact from the same batch, --model and taxonomy
version, with no --refresh to bypass it -- but never writes under it
itself. get-stats writes a stats_<name> file under --artifacts-dir only
when the flag is given; it has no default, and leaving it unset means the
aggregate is only printed.

Connection: set GITLAB_URL in the environment (default https://gitlab.com
when unset) and GITLAB_TOKEN (required; scope read_user or api). Results
are filtered by whatever the token can see.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		newFindMRsCmd(),
		newGetCommentsCmd(),
		newGetClassifyBatchCmd(),
		newSaveLabelsCmd(),
		newFilterCommentsCmd(),
		newGetStatsCmd(),
	)

	return root
}
