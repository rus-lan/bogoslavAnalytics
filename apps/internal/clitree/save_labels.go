package clitree

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

// saveLabelsFlags holds the raw --flag values for save-labels before
// they are validated and converted into an app.SaveLabelsRequest.
type saveLabelsFlags struct {
	fromArtifact string
	taxonomyFile string
	labels       string
	tool         string
	model        string
	classifiedAt string

	out commonOutputFlags
}

// newSaveLabelsCmd builds the save-labels command: the CLI mirror of the
// save_labels MCP tool and app.SaveLabels (TZ.md sections 7.2, 7.3, 8.1).
func newSaveLabelsCmd() *cobra.Command {
	var flags saveLabelsFlags

	cmd := &cobra.Command{
		Use:   "save-labels",
		Short: "Validate a labeling result and write it as a labeled_comments artifact",
		Long: `save-labels validates a labeling result -- produced by the calling
agent, never by this command -- against the comment_list batch it was
produced for and the taxonomy, and only on success writes artifact-3
(labeled_comments) with the mandatory classifier provenance block (TZ.md
section 8.3). A labeling that fails validation (a label outside the
taxonomy, an extra, missing, or duplicate note_id) writes nothing: every
violation is reported, not just the first.

--labels is a JSON file (or "-" for stdin) holding an array of
{"note_id": <integer>, "label": "<taxonomy label>"} entries, one per
comment in the batch, with none left out, none repeated, and none added
that was not in the batch.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSaveLabels(cmd, flags)
		},
	}

	registerSaveLabelsFlags(cmd, &flags)

	return cmd
}

// registerSaveLabelsFlags registers every save-labels flag on cmd,
// storing values in flags. It is split out of newSaveLabelsCmd so tests
// can build a throwaway command, parse args into a flags value, and
// check newSaveLabelsRequest's mapping without going through cobra's
// Execute.
func registerSaveLabelsFlags(cmd *cobra.Command, flags *saveLabelsFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.fromArtifact, "from-artifact", "", "path to the comment_list artifact the labeling was produced for (required)")
	fs.StringVar(&flags.taxonomyFile, "taxonomy-file", "", "path to the same custom taxonomy JSON file get-classify-batch handed out; default is the built-in v1 taxonomy")
	fs.StringVar(&flags.labels, "labels", "", `path to a JSON file with the labeling result, or "-" to read it from stdin (required)`)
	fs.StringVar(&flags.tool, "tool", "", "name of the tool that ran the labeling, for the classifier provenance block (required)")
	fs.StringVar(&flags.model, "model", "", "model that ran the labeling, for the classifier provenance block (required)")
	fs.StringVar(&flags.classifiedAt, "classified-at", "", "RFC 3339 timestamp the labeling was produced at, for the classifier provenance block (default: now)")
	_ = cmd.MarkFlagRequired("from-artifact")
	_ = cmd.MarkFlagRequired("labels")
	_ = cmd.MarkFlagRequired("tool")
	_ = cmd.MarkFlagRequired("model")

	addCommonOutputFlags(cmd, &flags.out, formatFourKinds, dirNoCache)
}

// newSaveLabelsRequest converts flags, an already-read labeling result
// and a resolved classifiedAt timestamp into an app.SaveLabelsRequest. It
// does not read --labels or --taxonomy-file itself: runSaveLabels does,
// so this mapping stays pure and testable on its own.
func newSaveLabelsRequest(flags saveLabelsFlags, labels []classify.NoteLabel, taxonomy *classify.Taxonomy, classifiedAt time.Time) (app.SaveLabelsRequest, error) {
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return app.SaveLabelsRequest{}, err
	}
	return app.SaveLabelsRequest{
		CommentListPath: flags.fromArtifact,
		Taxonomy:        taxonomy,
		Labels:          labels,
		Tool:            flags.tool,
		Model:           flags.model,
		ClassifiedAt:    classifiedAt,
		Dir:             flags.out.dir,
		Format:          format,
	}, nil
}

// runSaveLabels reads --labels and --taxonomy-file, builds the request,
// calls app.SaveLabels (TZ.md section 7.2: one function of the internal
// package per command), and renders the result.
func runSaveLabels(cmd *cobra.Command, flags saveLabelsFlags) error {
	taxonomy, err := readTaxonomyFile(flags.taxonomyFile)
	if err != nil {
		return err
	}

	labels, err := readNoteLabels(cmd, flags.labels)
	if err != nil {
		return err
	}

	classifiedAt := time.Now()
	if flags.classifiedAt != "" {
		classifiedAt, err = time.Parse(time.RFC3339, flags.classifiedAt)
		if err != nil {
			return fmt.Errorf("--classified-at: %w", err)
		}
	}

	req, err := newSaveLabelsRequest(flags, labels, taxonomy, classifiedAt)
	if err != nil {
		return err
	}

	result, err := app.SaveLabels(req)
	if err != nil {
		return fmt.Errorf("save-labels: %w", err)
	}

	return writeArtifactResult(cmd, result.Path, flags.out.out)
}
