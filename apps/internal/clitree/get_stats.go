package clitree

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
)

// getStatsFlags holds the raw --flag values for get-stats before they
// are validated and converted into an app.GetStatsRequest.
type getStatsFlags struct {
	fromArtifact string

	out commonOutputFlags
}

// newGetStatsCmd builds the get-stats command: the CLI mirror of the
// get_stats MCP tool and app.GetStats (TZ.md sections 7.2, 7.2.1, 7.3).
func newGetStatsCmd() *cobra.Command {
	var flags getStatsFlags

	cmd := &cobra.Command{
		Use:   "get-stats",
		Short: "Aggregate the items of any one artifact (mr_list, comment_list, labeled_comments, or filtered_comments)",
		Long: `get-stats reads --from-artifact -- any one of the four artifact
kinds -- and aggregates it: total item count, a breakdown by merge request
(comment_list, labeled_comments, filtered_comments only), by label
(labeled_comments, filtered_comments only), and by day of --from-artifact's
own created_at. get-stats never calls GitLab: it only aggregates an
already-written artifact's items.

Without --artifacts-dir, the aggregate is only printed (json or yaml; it is
not one of the four artifact kinds and has no text or html rendering of
its own). With --artifacts-dir, it is also written as a
stats_<name>.<ext> file under that directory (json or yaml only).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetStats(cmd, flags)
		},
	}

	registerGetStatsFlags(cmd, &flags)

	return cmd
}

// registerGetStatsFlags registers every get-stats flag on cmd, storing
// values in flags. It is split out of newGetStatsCmd so tests can build a
// throwaway command, parse args into a flags value, and check
// newGetStatsRequest's mapping without going through cobra's Execute.
func registerGetStatsFlags(cmd *cobra.Command, flags *getStatsFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.fromArtifact, "from-artifact", "", "path to the artifact to aggregate (required)")
	_ = cmd.MarkFlagRequired("from-artifact")

	addCommonOutputFlags(cmd, &flags.out, formatJSONYAMLOnly, dirStatsOnly)
}

// newGetStatsRequest converts flags into an app.GetStatsRequest.
func newGetStatsRequest(flags getStatsFlags) (app.GetStatsRequest, error) {
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return app.GetStatsRequest{}, err
	}
	return app.GetStatsRequest{
		ArtifactPath: flags.fromArtifact,
		Dir:          flags.out.dir,
		Format:       format,
	}, nil
}

// runGetStats builds the request, calls app.GetStats (TZ.md section 7.2:
// one function of the internal package per command), and renders the
// result.
func runGetStats(cmd *cobra.Command, flags getStatsFlags) error {
	req, err := newGetStatsRequest(flags)
	if err != nil {
		return err
	}

	result, err := app.GetStats(req)
	if err != nil {
		return fmt.Errorf("get-stats: %w", err)
	}

	if result.Path != "" {
		return writeArtifactResult(cmd, result.Path, flags.out.out)
	}

	if req.Format != artifact.FormatJSON && req.Format != artifact.FormatYAML {
		return fmt.Errorf("--format %q: without --artifacts-dir, get-stats only supports json or yaml (stats is not one of the four artifact kinds and has no text or html rendering)", req.Format)
	}

	data, err := marshalJSONOrYAML(req.Format, result.Stats)
	if err != nil {
		return fmt.Errorf("get-stats: render: %w", err)
	}
	return writeResult(cmd, flags.out.out, data)
}
