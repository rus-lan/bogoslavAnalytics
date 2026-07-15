package main

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/app"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/mcptool"
)

// newSaveLabelsRequest converts in, and a resolved classifiedAt
// timestamp, into an app.SaveLabelsRequest.
func newSaveLabelsRequest(in mcptool.SaveLabelsInput) (app.SaveLabelsRequest, error) {
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
func (s *toolServer) saveLabels(_ context.Context, _ *mcp.CallToolRequest, in mcptool.SaveLabelsInput) (*mcp.CallToolResult, mcptool.SaveLabelsOutput, error) {
	req, err := newSaveLabelsRequest(in)
	if err != nil {
		return nil, mcptool.SaveLabelsOutput{}, err
	}

	result, err := app.SaveLabels(req)
	if err != nil {
		return nil, mcptool.SaveLabelsOutput{}, fmt.Errorf("save_labels: %w", err)
	}

	return nil, mcptool.SaveLabelsOutput{Path: result.Path, Count: len(result.Doc.Items)}, nil
}
