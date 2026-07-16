package clitree

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// findMRsFlags holds the raw --flag values for find-mrs before they are
// validated and converted into an app.FindMRsRequest.
type findMRsFlags struct {
	user     string
	from     string
	to       string
	moreThan int
	group    string
	project  string
	mr       int64
	strict   bool
	timeout  time.Duration

	out   commonOutputFlags
	cache cacheFlags
}

// newFindMRsCmd builds the find-mrs command: the CLI mirror of the
// find_mrs MCP tool and app.FindMRs (TZ.md sections 7.2, 7.3).
func newFindMRsCmd() *cobra.Command {
	var flags findMRsFlags

	cmd := &cobra.Command{
		Use:   "find-mrs",
		Short: "Find merge requests where a user left more than N comments",
		Long: `find-mrs finds merge requests where --user left STRICTLY more than
--more-than comments in the [--from, --to] date range: a merge request with
exactly --more-than comments is NOT returned, only one with --more-than + 1
or more.

Candidates come from one of two strategies, chosen automatically by an
autoselector -- events (the fast primary path) or bruteforce (the slower,
always-correct fallback) -- reported on stderr together with the smoke test
result that gates the choice, since neither is something the user chose
directly. --strict forces bruteforce.

Point mode: pass --mr together with --project to return exactly that one
merge request, with no candidate search of any kind.

The result is artifact-1 (mr_list), written under --artifacts-dir in
--format. That same file is the cache for the next identical request
(json/yaml only; --refresh bypasses it); a cache hit is reported on stderr.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindMRs(cmd, flags)
		},
	}

	registerFindMRsFlags(cmd, &flags)

	return cmd
}

// registerFindMRsFlags registers every find-mrs flag on cmd, storing
// values in flags. It is split out of newFindMRsCmd so tests can build a
// throwaway command, parse args into a flags value, and check
// newFindMRsRequest's mapping without going through cobra's Execute.
func registerFindMRsFlags(cmd *cobra.Command, flags *findMRsFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.user, "user", "", "GitLab username or numeric user id (required)")
	fs.StringVar(&flags.from, "from", "", `start of the date range, inclusive, "YYYY-MM-DD" (required)`)
	fs.StringVar(&flags.to, "to", "", `end of the date range, inclusive, "YYYY-MM-DD" (required)`)
	fs.IntVar(&flags.moreThan, "more-than", 0, "N: return only merge requests with STRICTLY more than N comments by --user (exactly N is excluded)")
	fs.StringVar(&flags.group, "group", "", "restrict the search to this group's projects, including subgroups (numeric id or path)")
	fs.StringVar(&flags.project, "project", "", "restrict the search to this single project (numeric id or path); required together with --mr")
	fs.Int64Var(&flags.mr, "mr", 0, "point mode: return exactly this merge request iid, with no candidate search; requires --project")
	fs.BoolVar(&flags.strict, "strict", false, "force the bruteforce strategy, skipping the events strategy and its smoke test")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")

	addCommonOutputFlags(cmd, &flags.out, formatFourKinds, dirCachedRefresh)
	addCacheFlags(cmd, &flags.cache)
	addTimeoutFlag(cmd, &flags.timeout)
}

// newFindMRsRequest converts flags into an app.FindMRsRequest. It makes
// no GitLab call and does not resolve --user: app.FindMRs does both
// itself (TZ.md section 5.0), so this mapping stays pure and testable on
// its own.
func newFindMRsRequest(cmd *cobra.Command, flags findMRsFlags, gitlabURL string) (app.FindMRsRequest, error) {
	from, err := domain.ParseDate(flags.from)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("--from: %w", err)
	}
	to, err := domain.ParseDate(flags.to)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("--to: %w", err)
	}
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return app.FindMRsRequest{}, err
	}

	req := app.FindMRsRequest{
		GitlabURL: gitlabURL,
		User:      flags.user,
		From:      from,
		To:        to,
		MoreThan:  flags.moreThan,
		Group:     flags.group,
		Project:   flags.project,
		Strict:    flags.strict,
		Dir:       flags.out.dir,
		Format:    format,
		Cache:     app.CacheOptions{TTL: flags.cache.ttl, Refresh: flags.cache.refresh},
	}
	if cmd.Flags().Changed("mr") {
		mr := flags.mr
		req.MR = &mr
	}
	return req, nil
}

// runFindMRs builds the request, calls app.FindMRs (TZ.md section 7.2:
// one function of the internal package per command), and renders the
// result. Point mode's "--mr requires --project" rule (TZ.md sections
// 1.2, 7.2) is enforced by app.FindMRs itself, not here: it is the very
// first thing FindMRs checks, before it ever touches client, so it never
// needs a network call to surface.
func runFindMRs(cmd *cobra.Command, flags findMRsFlags) error {
	req, err := newFindMRsRequest(cmd, flags, resolvedGitlabURL())
	if err != nil {
		return err
	}

	opts, err := timeoutOption(cmd, flags.timeout)
	if err != nil {
		return err
	}
	client, err := newGitlabClient(opts...)
	if err != nil {
		return err
	}

	result, err := app.FindMRs(cmd.Context(), client, req)
	if err != nil {
		return fmt.Errorf("find-mrs: %w", err)
	}

	reportCacheHit(cmd, result.CacheHit, result.Path)
	if result.CacheHit {
		reportFormatMismatch(cmd, req.Format, result.Path)
	}
	reportStrategy(cmd, result.Doc.Query)

	return writeArtifactResult(cmd, result.Path, flags.out.out)
}
