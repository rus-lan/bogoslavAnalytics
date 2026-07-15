package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

// SaveLabelsInput is the save_labels tool's input: the MCP mirror of
// bogoslav-cli's save-labels command and app.SaveLabelsRequest (TZ.md
// sections 7.2, 7.3, 8.1). Unlike bogoslav-cli (which reads --labels
// from a file or stdin), labels arrive inline as structured data: the
// calling agent already holds them as the result of the labeling
// get_classify_batch asked it to run.
type SaveLabelsInput struct {
	FromArtifact string               `json:"from_artifact" jsonschema:"path to the comment_list artifact (json or yaml only) the labeling was produced for"`
	Labels       []classify.NoteLabel `json:"labels" jsonschema:"the labeling result: one {note_id, label} entry per comment in the batch, with none left out, none repeated, and none added that was not in the batch"`
	Taxonomy     *classify.Taxonomy   `json:"taxonomy,omitempty" jsonschema:"the exact same taxonomy get_classify_batch handed out for this batch; omit to validate against the built-in default taxonomy"`
	Tool         string               `json:"tool" jsonschema:"name of the tool that ran the labeling, recorded in the mandatory classifier provenance block"`
	Model        string               `json:"model" jsonschema:"model that ran the labeling, recorded in the mandatory classifier provenance block"`
	ClassifiedAt string               `json:"classified_at,omitempty" jsonschema:"RFC 3339 timestamp the labeling was produced at, recorded in the classifier provenance block; omit to use the current time"`

	ArtifactsDir string `json:"artifacts_dir,omitempty" jsonschema:"directory the labeled_comments artifact is written under (default \"artifacts\")"`
	Format       string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml). A labeling result that fails validation -- a label outside the taxonomy, an extra, missing, or duplicate note_id -- writes NO file, in any of the four formats, and returns an error listing every violation"`
}

// SaveLabelsOutput is the save_labels tool's output.
type SaveLabelsOutput struct {
	Path  string `json:"path" jsonschema:"path to the written labeled_comments artifact"`
	Count int    `json:"count" jsonschema:"number of labeled comments written"`
}

// newSaveLabelsRequest converts in, and a resolved classifiedAt
// timestamp, into an app.SaveLabelsRequest.
func newSaveLabelsRequest(in SaveLabelsInput) (app.SaveLabelsRequest, error) {
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.SaveLabelsRequest{}, err
	}

	classifiedAt := time.Now()
	if in.ClassifiedAt != "" {
		classifiedAt, err = time.Parse(time.RFC3339, in.ClassifiedAt)
		if err != nil {
			return app.SaveLabelsRequest{}, fmt.Errorf("classified_at: %w", err)
		}
	}

	return app.SaveLabelsRequest{
		CommentListPath: in.FromArtifact,
		Taxonomy:        in.Taxonomy,
		Labels:          in.Labels,
		Tool:            in.Tool,
		Model:           in.Model,
		ClassifiedAt:    classifiedAt,
		Dir:             in.ArtifactsDir,
		Format:          format,
	}, nil
}

// saveLabels is the save_labels tool handler: validate the incoming
// labeling and call app.SaveLabels (TZ.md section 7.3: one function of
// apps/internal/app per tool). A labeling that fails
// classify.Validate returns the error and writes nothing: app.SaveLabels
// never reaches artifact.WriteLabeledComments on that path (TZ.md
// section 8.1).
func (s *toolServer) saveLabels(_ context.Context, _ *mcp.CallToolRequest, in SaveLabelsInput) (*mcp.CallToolResult, SaveLabelsOutput, error) {
	req, err := newSaveLabelsRequest(in)
	if err != nil {
		return nil, SaveLabelsOutput{}, err
	}

	result, err := app.SaveLabels(req)
	if err != nil {
		return nil, SaveLabelsOutput{}, fmt.Errorf("save_labels: %w", err)
	}

	return nil, SaveLabelsOutput{Path: result.Path, Count: len(result.Doc.Items)}, nil
}
