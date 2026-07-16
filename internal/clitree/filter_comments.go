package clitree

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// filterCommentsFlags holds the raw --flag values for filter-comments
// before they are validated and converted into an
// app.FilterCommentsRequest.
type filterCommentsFlags struct {
	fromArtifact string
	labels       []string
	from         string
	to           string
	group        string
	project      string
	timeout      time.Duration

	out commonOutputFlags
}

// newFilterCommentsCmd builds the filter-comments command: the CLI
// mirror of the filter_comments MCP tool and app.FilterComments (TZ.md
// sections 7.2, 7.3).
func newFilterCommentsCmd() *cobra.Command {
	var flags filterCommentsFlags

	cmd := &cobra.Command{
		Use:   "filter-comments",
		Short: "Narrow a labeled_comments artifact down to a set of labels",
		Long: `filter-comments reads an existing labeled_comments artifact and keeps
only the rows whose label is one of --label, with optional further
narrowing by date range (--from and --to, both or neither) and by --group
or --project.

--group and --project narrow by GitLab's numeric project ids, which this
command resolves for you: --group calls GET /groups/:id/projects to list
every project id in the group (including subgroups); --project resolves to
a single numeric id directly if it is already numeric, or with one
GET /projects/:id call if it is a namespaced path. Both need GITLAB_TOKEN;
neither is needed at all if you pass neither flag.

The result is artifact-4 (filtered_comments), written under
--artifacts-dir in --format. filter-comments never consults a cache before
running: it always reads --from-artifact and reprocesses it.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFilterComments(cmd, flags)
		},
	}

	registerFilterCommentsFlags(cmd, &flags)

	return cmd
}

// registerFilterCommentsFlags registers every filter-comments flag on
// cmd, storing values in flags. It is split out of newFilterCommentsCmd
// so tests can build a throwaway command, parse args into a flags value,
// and check newFilterCommentsRequest's mapping without going through
// cobra's Execute.
func registerFilterCommentsFlags(cmd *cobra.Command, flags *filterCommentsFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.fromArtifact, "from-artifact", "", "path to the labeled_comments artifact to filter (required)")
	fs.StringArrayVar(&flags.labels, "label", nil, "taxonomy label to keep (repeatable; at least one required)")
	fs.StringVar(&flags.from, "from", "", `start of an additional date filter, inclusive, "YYYY-MM-DD"; requires --to`)
	fs.StringVar(&flags.to, "to", "", `end of an additional date filter, inclusive, "YYYY-MM-DD"; requires --from`)
	fs.StringVar(&flags.group, "group", "", "keep only comments on merge requests in this group's projects, including subgroups (numeric id or path)")
	fs.StringVar(&flags.project, "project", "", "keep only comments on merge requests in this single project (numeric id or path)")
	_ = cmd.MarkFlagRequired("from-artifact")

	addCommonOutputFlags(cmd, &flags.out, formatFourKinds, dirNoCache)
	addTimeoutFlag(cmd, &flags.timeout)
}

// resolveFilterScope resolves --group/--project into the numeric ids
// app.FilterCommentsRequest actually filters by (TZ.md section 7.2, and
// FilterCommentsRequest's own doc comment: "a caller resolves ahead of
// time ... if it wants --group/--project filtering to actually narrow
// anything"). It calls GitLab only for whichever of the two flags is
// non-empty.
func resolveFilterScope(ctx context.Context, client *gitlab.Client, group, project string) (projectIDs []int64, projectID *int64, err error) {
	if project != "" {
		id, err := resolveProjectID(ctx, client, project)
		if err != nil {
			return nil, nil, err
		}
		projectID = &id
	}
	if group != "" {
		projects, err := client.GroupProjects(ctx, buildGitlabID(group))
		if err != nil {
			return nil, nil, fmt.Errorf("resolve --group %q: %w", group, err)
		}
		projectIDs = make([]int64, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}
	}
	return projectIDs, projectID, nil
}

// resolveProjectID resolves a --project value to its numeric id: parsed
// directly if it is already all digits, or via one GetProject call if it
// is a namespaced path.
func resolveProjectID(ctx context.Context, client *gitlab.Client, project string) (int64, error) {
	if n, ok := parseNumericID(project); ok {
		return n, nil
	}
	p, err := client.GetProject(ctx, gitlab.PathID(project))
	if err != nil {
		return 0, fmt.Errorf("resolve --project %q: %w", project, err)
	}
	return p.ID, nil
}

// parseOptionalDateRange parses --from/--to into the *domain.Date pair
// app.FilterCommentsRequest carries: both empty means no extra date
// filter (nil, nil); app.FilterComments itself rejects exactly one being
// set (ErrIncompleteDateFilter), so that check is not repeated here.
func parseOptionalDateRange(from, to string) (*domain.Date, *domain.Date, error) {
	if from == "" && to == "" {
		return nil, nil, nil
	}
	var f, t domain.Date
	var err error
	if from != "" {
		if f, err = domain.ParseDate(from); err != nil {
			return nil, nil, fmt.Errorf("--from: %w", err)
		}
	}
	if to != "" {
		if t, err = domain.ParseDate(to); err != nil {
			return nil, nil, fmt.Errorf("--to: %w", err)
		}
	}
	return dateOrNil(from, f), dateOrNil(to, t), nil
}

// dateOrNil returns nil when raw is empty, or a pointer to parsed
// otherwise.
func dateOrNil(raw string, parsed domain.Date) *domain.Date {
	if raw == "" {
		return nil
	}
	return &parsed
}

// newFilterCommentsRequest converts flags and already-resolved
// --group/--project scope into an app.FilterCommentsRequest. It makes no
// GitLab call itself: resolveFilterScope is runFilterComments's job, so
// this mapping stays pure and testable on its own.
func newFilterCommentsRequest(flags filterCommentsFlags, from, to *domain.Date, projectIDs []int64, projectID *int64) (app.FilterCommentsRequest, error) {
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return app.FilterCommentsRequest{}, err
	}
	return app.FilterCommentsRequest{
		LabeledCommentsPath: flags.fromArtifact,
		Labels:              flags.labels,
		From:                from,
		To:                  to,
		Group:               flags.group,
		Project:             flags.project,
		ProjectIDs:          projectIDs,
		ProjectID:           projectID,
		Dir:                 flags.out.dir,
		Format:              format,
	}, nil
}

// runFilterComments resolves --group/--project (when set), builds the
// request, calls app.FilterComments (TZ.md section 7.2: one function of
// the internal package per command), and renders the result.
func runFilterComments(cmd *cobra.Command, flags filterCommentsFlags) error {
	from, to, err := parseOptionalDateRange(flags.from, flags.to)
	if err != nil {
		return err
	}

	var projectIDs []int64
	var projectID *int64
	if flags.group != "" || flags.project != "" {
		opts, err := timeoutOption(cmd, flags.timeout)
		if err != nil {
			return err
		}
		client, err := newGitlabClient(opts...)
		if err != nil {
			return err
		}
		projectIDs, projectID, err = resolveFilterScope(cmd.Context(), client, flags.group, flags.project)
		if err != nil {
			return fmt.Errorf("filter-comments: %w", err)
		}
	}

	req, err := newFilterCommentsRequest(flags, from, to, projectIDs, projectID)
	if err != nil {
		return err
	}

	result, err := app.FilterComments(req)
	if err != nil {
		return fmt.Errorf("filter-comments: %w", err)
	}

	return writeArtifactResult(cmd, result.Path, flags.out.out)
}
