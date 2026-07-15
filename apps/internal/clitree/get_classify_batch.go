package clitree

import (
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// getClassifyBatchFlags holds the raw --flag values for
// get-classify-batch before they are validated and converted into an
// app.GetClassifyBatchRequest.
type getClassifyBatchFlags struct {
	fromArtifact string
	model        string
	taxonomyFile string

	out commonOutputFlags
}

// classifyBatchOutput is what get-classify-batch prints on a cache miss:
// everything the calling agent needs to label the batch itself (TZ.md
// section 8.1). It is not one of the four cached artifact kinds, so it
// carries no schema_version/source/query and is rendered only as json or
// yaml (see runGetClassifyBatch).
type classifyBatchOutput struct {
	Batch    []domain.Note      `json:"batch"`
	Taxonomy classify.Taxonomy  `json:"taxonomy"`
	Schema   *jsonschema.Schema `json:"schema"`
	Prompt   string             `json:"prompt"`
}

// newGetClassifyBatchCmd builds the get-classify-batch command: the CLI
// mirror of the get_classify_batch MCP tool and app.GetClassifyBatch
// (TZ.md sections 7.2, 7.3, 8.1). It never labels anything itself: the
// calling agent does that, using the batch, taxonomy, schema and prompt
// this command hands back.
func newGetClassifyBatchCmd() *cobra.Command {
	var flags getClassifyBatchFlags

	cmd := &cobra.Command{
		Use:   "get-classify-batch",
		Short: "Hand back a batch of comments for the calling agent to label",
		Long: `get-classify-batch reads an existing comment_list artifact and hands
back everything the calling agent needs to label it: the batch of comments,
the taxonomy, the labeling result's JSON Schema, and a rendered prompt.
get-classify-batch never labels anything itself and never calls an LLM
(TZ.md section 8.1): the calling agent labels, and passes its result to
save-labels.

If an unchanged batch (same comments, same --model, same taxonomy version)
already has a matching labeled_comments artifact, that is reported instead
(cache hit, on stderr) and no batch is handed out.

The non-cached batch/taxonomy/schema/prompt output is not one of the four
cached artifact kinds, so --format only accepts json or yaml here (it has
no text or html rendering of its own).`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetClassifyBatch(cmd, flags)
		},
	}

	registerGetClassifyBatchFlags(cmd, &flags)

	return cmd
}

// registerGetClassifyBatchFlags registers every get-classify-batch flag
// on cmd, storing values in flags. It is split out of
// newGetClassifyBatchCmd so tests can build a throwaway command, parse
// args into a flags value, and check newGetClassifyBatchRequest's
// mapping without going through cobra's Execute.
func registerGetClassifyBatchFlags(cmd *cobra.Command, flags *getClassifyBatchFlags) {
	fs := cmd.Flags()
	fs.StringVar(&flags.fromArtifact, "from-artifact", "", "path to the comment_list artifact to read the batch from (required)")
	fs.StringVar(&flags.model, "model", "", "model identifier the calling agent will label with; recorded for the labeling cache key (required)")
	fs.StringVar(&flags.taxonomyFile, "taxonomy-file", "", "path to a custom taxonomy JSON file; default is the built-in v1 taxonomy")
	_ = cmd.MarkFlagRequired("from-artifact")
	_ = cmd.MarkFlagRequired("model")

	addCommonOutputFlags(cmd, &flags.out, formatJSONYAMLOnly, dirCachedNoRefresh)
}

// newGetClassifyBatchRequest converts flags into an
// app.GetClassifyBatchRequest.
func newGetClassifyBatchRequest(flags getClassifyBatchFlags) (app.GetClassifyBatchRequest, error) {
	taxonomy, err := readTaxonomyFile(flags.taxonomyFile)
	if err != nil {
		return app.GetClassifyBatchRequest{}, err
	}
	return app.GetClassifyBatchRequest{
		CommentListPath: flags.fromArtifact,
		Model:           flags.model,
		Taxonomy:        taxonomy,
		Dir:             flags.out.dir,
	}, nil
}

// runGetClassifyBatch builds the request, calls app.GetClassifyBatch
// (TZ.md section 7.2: one function of the internal package per command),
// and renders the result.
func runGetClassifyBatch(cmd *cobra.Command, flags getClassifyBatchFlags) error {
	format, err := parseFormat(flags.out.format)
	if err != nil {
		return err
	}

	req, err := newGetClassifyBatchRequest(flags)
	if err != nil {
		return err
	}

	result, err := app.GetClassifyBatch(req)
	if err != nil {
		return fmt.Errorf("get-classify-batch: %w", err)
	}

	if result.Cached {
		reportCacheHit(cmd, true, result.ArtifactPath)
		reportFormatMismatch(cmd, format, result.ArtifactPath)
		return writeArtifactResult(cmd, result.ArtifactPath, flags.out.out)
	}

	if format != artifact.FormatJSON && format != artifact.FormatYAML {
		return fmt.Errorf("--format %q: the batch/taxonomy/schema/prompt output only supports json or yaml (it is not one of the four cached artifact kinds, so it has no text or html rendering)", format)
	}

	data, err := marshalJSONOrYAML(format, classifyBatchOutput{
		Batch:    result.Batch,
		Taxonomy: result.Taxonomy,
		Schema:   result.Schema,
		Prompt:   result.Prompt,
	})
	if err != nil {
		return fmt.Errorf("get-classify-batch: render: %w", err)
	}

	return writeResult(cmd, flags.out.out, data)
}
