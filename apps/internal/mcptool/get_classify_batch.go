package mcptool

import (
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// GetClassifyBatchInput is the get_classify_batch tool's input: the MCP
// mirror of bogoslav-cli's get-classify-batch command and
// app.GetClassifyBatchRequest (TZ.md sections 7.2, 7.3, 8.1). This tool
// never labels anything itself and never calls a model: the calling
// agent labels, using the batch, taxonomy, schema and prompt handed back
// here, and passes its result to save_labels.
type GetClassifyBatchInput struct {
	FromArtifact string             `json:"from_artifact" jsonschema:"path to the comment_list artifact (json or yaml only) to read the labeling batch from"`
	Model        string             `json:"model" jsonschema:"model identifier the calling agent will run to label this batch; this tool never calls the model itself -- the identifier is only recorded, for the labeled_comments artifact's classifier provenance block and as part of the labeling cache key"`
	Taxonomy     *classify.Taxonomy `json:"taxonomy,omitempty" jsonschema:"custom taxonomy to label against; omit to use the built-in default taxonomy"`
	ArtifactsDir string             `json:"artifacts_dir,omitempty" jsonschema:"directory an existing matching labeled_comments artifact is looked up in, to answer with cached=true instead of handing out a new batch (default \"artifacts\")"`
}

// GetClassifyBatchOutput is the get_classify_batch tool's output: either
// a cache hit naming an existing labeled_comments artifact, or a fresh
// batch plus the taxonomy, result JSON Schema and rendered prompt the
// calling agent needs to label it.
//
// This type is registered with an "any" output type in AddTool (see
// apps/cmd/bogoslav-mcp/server.go), not itself: Schema's own Go type
// (*github.com/google/jsonschema-go/jsonschema.Schema) is self-referential
// (Defs/Properties/Items all point back to Schema), and jsonschema.For /
// jsonschema.ForType -- the same function AddTool would otherwise use to
// infer an output schema -- errors on that cycle ("cycle detected for
// type jsonschema.Schema") rather than emitting a $ref. Marshaling an
// actual *jsonschema.Schema *value* to JSON (what happens at call time)
// is unaffected: only the type-level schema-of-a-schema inference is
// impossible for this shape.
type GetClassifyBatchOutput struct {
	Cached       bool               `json:"cached" jsonschema:"true when an existing labeled_comments artifact already matches this batch, model, and taxonomy version; when true, artifact_path is set and batch/taxonomy/schema/prompt are omitted"`
	ArtifactPath string             `json:"artifact_path,omitempty" jsonschema:"path to the existing labeled_comments artifact; set only when cached is true"`
	Batch        []domain.Note      `json:"batch,omitempty" jsonschema:"comments to label; set only when cached is false"`
	Taxonomy     classify.Taxonomy  `json:"taxonomy,omitempty" jsonschema:"taxonomy the calling agent must label against; set only when cached is false"`
	Schema       *jsonschema.Schema `json:"schema,omitempty" jsonschema:"JSON Schema (draft 2020-12) the calling agent's labeling result must satisfy: an array of {note_id, label} objects; set only when cached is false"`
	Prompt       string             `json:"prompt,omitempty" jsonschema:"rendered instructions for the calling agent's model to label the batch; this tool never calls a model itself; set only when cached is false"`
}
