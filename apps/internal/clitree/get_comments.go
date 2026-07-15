package clitree

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// getCommentsFlags holds the raw --flag values for get-comments before
// they are validated and converted into an app.GetCommentsRequest.
type getCommentsFlags struct {
	user         string
	from         string
	to           string
	fromArtifact string
	project      int64
	mrs          []int64

	out   commonOutputFlags
	cache cacheFlags
}

// newGetCommentsCmd builds the get-comments command: the CLI mirror of
// the get_comments MCP tool and app.GetComments (TZ.md sections 7.2, 7.3).
func newGetCommentsCmd() *cobra.Command {
	var flags getCommentsFlags

	cmd := &cobra.Command{
		Use:   "get-comments",
		Short: "Fetch the comments a user left across a set of merge requests",
		Long: `get-comments fetches every comment --user left, inside [--from, --to],
across a set of merge requests -- one /discussions call per merge request,
the single most call-expensive step in the pipeline.

The merge request set comes from exactly one of two places: --from-artifact
(an existing mr_list file, typically find-mrs's own output), or an explicit
list built from --project together with one or more --mr flags, all naming
merge requests in that one project. Naming merge requests across more than
one project without a shared mr_list artifact is not supported here; run
find-mrs first instead.

The result is artifact-2 (comment_list), written under --artifacts-dir in
--format. That same file is the cache for the next identical request
(json/yaml only; --refresh bypasses it); a cache hit is reported on stderr.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetComments(cmd, flags)
		},
	}

	registerGetCommentsFlags(cmd, &flags)

	return cmd
}

// registerGetCommentsFlags registers every get-comments flag on cmd,
// storing values in flags. It is split out of newGetCommentsCmd so tests
// can build a throwaway command, parse args into a flags value, and
// check newGetCommentsRequest's mapping without going through cobra's
// Execute.
func registerGetCommentsFlags(cmd *cobra.Command, flags *getCommentsFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.user, "user", "", "GitLab username or numeric user id (required)")
	fs.StringVar(&flags.from, "from", "", `start of the date range, inclusive, "YYYY-MM-DD" (required)`)
	fs.StringVar(&flags.to, "to", "", `end of the date range, inclusive, "YYYY-MM-DD" (required)`)
	fs.StringVar(&flags.fromArtifact, "from-artifact", "", "path to an existing mr_list artifact whose merge requests to fetch comments for; mutually exclusive with --project/--mr")
	fs.Int64Var(&flags.project, "project", 0, "numeric project id for an explicit merge request list, together with --mr; mutually exclusive with --from-artifact")
	fs.Int64SliceVar(&flags.mrs, "mr", nil, "merge request iid to fetch comments for (repeatable); requires --project")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("from")
	_ = cmd.MarkFlagRequired("to")

	addCommonOutputFlags(cmd, &flags.out, formatFourKinds, dirCachedRefresh)
	addCacheFlags(cmd, &flags.cache)
}

// buildMRRefs turns --project and --mr into the explicit merge request
// list app.GetCommentsRequest.MRs expects (TZ.md section 4.2): every --mr
// value paired with the one --project. Returns nil, nil when no --mr was
// given, so the caller falls through to --from-artifact.
//
// This is flag-shape validation, not a business rule about GitLab API
// behavior: artifact.MRRef.ProjectID is a plain int64, and an --mr
// without a --project to pair it with cannot be turned into one.
func buildMRRefs(cmd *cobra.Command, project int64, mrs []int64) ([]artifact.MRRef, error) {
	if len(mrs) == 0 {
		return nil, nil
	}
	if !cmd.Flags().Changed("project") {
		return nil, fmt.Errorf("--mr requires --project (a numeric project id) to build an explicit merge request list")
	}
	refs := make([]artifact.MRRef, len(mrs))
	for i, iid := range mrs {
		refs[i] = artifact.MRRef{ProjectID: project, MRIID: iid}
	}
	return refs, nil
}

// newGetCommentsRequest converts flags and an already-resolved userID
// into an app.GetCommentsRequest. It makes no GitLab call itself: userID
// resolution (TZ.md section 5.0) is runGetComments's job, so this mapping
// stays pure and testable on its own.
func newGetCommentsRequest(cmd *cobra.Command, flags getCommentsFlags, gitlabURL string, userID int64) (app.GetCommentsRequest, error) {
	from, err := domain.ParseDate(flags.from)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("--from: %w", err)
	}
	to, err := domain.ParseDate(flags.to)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("--to: %w", err)
	}
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return app.GetCommentsRequest{}, err
	}
	refs, err := buildMRRefs(cmd, flags.project, flags.mrs)
	if err != nil {
		return app.GetCommentsRequest{}, err
	}

	return app.GetCommentsRequest{
		GitlabURL:    gitlabURL,
		UserID:       userID,
		From:         from,
		To:           to,
		FromArtifact: flags.fromArtifact,
		MRs:          refs,
		Dir:          flags.out.dir,
		Format:       format,
		Cache:        app.CacheOptions{TTL: flags.cache.ttl, Refresh: flags.cache.refresh},
	}, nil
}

// runGetComments resolves --user, builds the request, calls
// app.GetComments (TZ.md section 7.2: one function of the internal
// package per command), and renders the result.
func runGetComments(cmd *cobra.Command, flags getCommentsFlags) error {
	client, err := newGitlabClient()
	if err != nil {
		return err
	}

	userID, err := app.ResolveUser(cmd.Context(), client, flags.user)
	if err != nil {
		return fmt.Errorf("get-comments: %w", err)
	}

	req, err := newGetCommentsRequest(cmd, flags, resolvedGitlabURL(), userID)
	if err != nil {
		return err
	}

	result, err := app.GetComments(cmd.Context(), client, req)
	if err != nil {
		return fmt.Errorf("get-comments: %w", err)
	}

	reportCacheHit(cmd, result.CacheHit, result.Path)
	if result.CacheHit {
		reportFormatMismatch(cmd, req.Format, result.Path)
	}

	return writeArtifactResult(cmd, result.Path, flags.out.out)
}
