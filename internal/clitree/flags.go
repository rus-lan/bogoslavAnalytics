package clitree

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// commonOutputFlags are the three flags every command in this tree
// carries (TZ.md section 3, section 4): --format chooses the wire
// format the result is rendered in; --artifacts-dir's exact meaning
// varies per command -- most write their result under it, some of those
// also look a matching artifact up there first as a cache, and
// get-classify-batch only ever reads from it -- so each command passes
// its own dirUsage describing exactly what it does, never a
// one-size-fits-all description; and --out redirects the copy of the
// result this command prints away from stdout and into a file instead.
type commonOutputFlags struct {
	format string
	dir    string
	out    string
}

// formatFourKinds is the --format help for commands whose result is one
// of the four cached artifact kinds (mr_list, comment_list,
// filtered_comments, labeled_comments): find-mrs, get-comments,
// filter-comments, save-labels. All four wire formats apply.
const formatFourKinds = "output format: json, yaml, text, or html. text and html are write-only: " +
	"they cannot be read back and cannot be passed to --from-artifact, and they are never a cache hit"

// formatJSONYAMLOnly is the --format help for commands whose result is
// not one of the four artifact kinds (get-stats, get-classify-batch):
// only json and yaml apply. Each command's Long help explains why.
const formatJSONYAMLOnly = "output format: json or yaml"

// dirCachedRefresh is the --artifacts-dir help for the two commands
// that both write a result under it and look a matching one up there
// first as a cache, with --refresh available to bypass that lookup
// (find-mrs, get-comments).
const dirCachedRefresh = `directory the result is written under, and where a matching json/yaml ` +
	`artifact is looked up as a cache first; --refresh bypasses that lookup (default "artifacts")`

// dirCachedNoRefresh is the --artifacts-dir help for get-classify-batch:
// unlike find-mrs and get-comments, it never writes anything under this
// directory itself -- on a cache miss it hands back a batch/taxonomy/
// schema/prompt for the calling agent to label, and save-labels is what
// eventually writes artifact-3, not this command (TZ.md section 8.1). It
// only looks a matching artifact up there first, as a cache, and has no
// --refresh flag to bypass that lookup (TZ.md section 8.4: the labeling
// cache is keyed by content, not by age).
const dirCachedNoRefresh = `directory a matching labeled_comments artifact from the same batch, ` +
	`--model and taxonomy version is looked up in as a cache; get-classify-batch never writes ` +
	`anything under this directory itself, and there is no --refresh flag to bypass that lookup ` +
	`(default "artifacts")`

// dirNoCache is the --artifacts-dir help for commands that always write
// a result under it but never look one up as a cache (filter-comments,
// save-labels).
const dirNoCache = `directory the result is written under (default "artifacts")`

// dirStatsOnly is the --artifacts-dir help for get-stats: unlike every
// other command, leaving it unset means the aggregate is only printed,
// never written.
const dirStatsOnly = `directory the aggregate is written under; without --artifacts-dir the ` +
	`aggregate is only printed`

// addCommonOutputFlags registers --format, --artifacts-dir and --out on
// cmd, storing their values in f. formatUsage and dirUsage are the
// per-command help text for --format and --artifacts-dir: what this
// specific command's result actually supports, not a one-size-fits-all
// description of the whole tree (TZ.md section 3: help text is
// user-facing product surface, generated verbatim into SKILL.md).
func addCommonOutputFlags(cmd *cobra.Command, f *commonOutputFlags, formatUsage, dirUsage string) {
	cmd.Flags().StringVar(&f.format, "format", string(artifact.FormatYAML), formatUsage)
	cmd.Flags().StringVar(&f.dir, "artifacts-dir", "", dirUsage)
	cmd.Flags().StringVar(&f.out, "out", "",
		"write the result to this file instead of stdout")
}

// cacheFlags are --refresh and --cache-ttl, wired only on the two
// commands whose app request carries app.CacheOptions (find-mrs,
// get-comments): every other command either never consults a cache
// (save-labels, filter-comments, get-stats) or has no adjustable TTL of
// its own (get-classify-batch), so adding these flags there would be
// dishonest -- a flag that looks like it does something but never does.
type cacheFlags struct {
	refresh bool
	ttl     time.Duration
}

// addCacheFlags registers --refresh and --cache-ttl on cmd, storing their
// values in f.
func addCacheFlags(cmd *cobra.Command, f *cacheFlags) {
	cmd.Flags().BoolVar(&f.refresh, "refresh", false,
		"bypass the cache and always call GitLab, even if a fresh cached artifact exists")
	cmd.Flags().DurationVar(&f.ttl, "cache-ttl", 0,
		`how long a cached artifact stays fresh, for example "24h" (default 24h)`)
}

// addTimeoutFlag registers --timeout on cmd, storing its value in d. It
// is only added to commands that build a GitLab client at all (find-mrs,
// get-comments, filter-comments): every other command never calls
// GitLab, so the flag would look like it does something but never does
// (the same reasoning addCacheFlags's doc comment already gives for
// --refresh/--cache-ttl).
//
// The flag's own zero value (unset) is deliberately indistinguishable
// from an explicit "--timeout 0s" only in the raw time.Duration -- the
// two are told apart by cmd.Flags().Changed("timeout"), in
// newGitlabClient's caller, not here.
func addTimeoutFlag(cmd *cobra.Command, d *time.Duration) {
	cmd.Flags().DurationVar(d, "timeout", 0,
		`per-request deadline for each call to GitLab -- one page of a listing, one `+
			`/discussions call, one retry attempt, not the whole command (covers connect, TLS, `+
			`sending the request, and reading the response), for example "60s" or "5m"; unset `+
			`uses BOGOSLAV_TIMEOUT if set, or else `+gitlab.DefaultTimeout.String()+`; `+
			`"0s" disables it entirely and waits as long as GitLab takes`)
}

// parseFormat validates s against the four artifact wire formats
// (TZ.md section 4). Every command runs its --format value through this
// before anything else, so an unknown value fails fast and clearly.
func parseFormat(s string) (artifact.Format, error) {
	switch f := artifact.Format(s); f {
	case artifact.FormatJSON, artifact.FormatYAML, artifact.FormatText, artifact.FormatHTML:
		return f, nil
	default:
		return "", fmt.Errorf("--format %q: must be one of json, yaml, text, html", s)
	}
}
