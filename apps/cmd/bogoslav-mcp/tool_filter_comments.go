package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/mcptool"
)

// FilterCommentsOutput is the filter_comments tool's output.
type FilterCommentsOutput struct {
	Path  string `json:"path" jsonschema:"path to the written filtered_comments artifact"`
	Count int    `json:"count" jsonschema:"number of comments kept after filtering"`
}

// newFilterCommentsRequest converts in and already-resolved group/
// project scope into an app.FilterCommentsRequest. It makes no GitLab
// call itself: resolveFilterScope is filterComments's job, so this
// mapping stays pure and testable on its own, mirroring bogoslav-cli's
// newFilterCommentsRequest.
func newFilterCommentsRequest(in mcptool.FilterCommentsInput, from, to *domain.Date, projectIDs []int64, projectID *int64) (app.FilterCommentsRequest, error) {
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.FilterCommentsRequest{}, err
	}
	return app.FilterCommentsRequest{
		LabeledCommentsPath: in.FromArtifact,
		Labels:              in.Labels,
		From:                from,
		To:                  to,
		Group:               in.Group,
		Project:             in.Project,
		ProjectIDs:          projectIDs,
		ProjectID:           projectID,
		Dir:                 in.ArtifactsDir,
		Format:              format,
	}, nil
}

// filterComments is the filter_comments tool handler: resolve group/
// project (when set), build the request, and call app.FilterComments
// (TZ.md section 7.3: one function of apps/internal/app per tool).
func (s *toolServer) filterComments(ctx context.Context, _ *mcp.CallToolRequest, in mcptool.FilterCommentsInput) (*mcp.CallToolResult, FilterCommentsOutput, error) {
	from, to, err := parseOptionalDateRange(in.From, in.To)
	if err != nil {
		return nil, FilterCommentsOutput{}, err
	}

	var projectIDs []int64
	var projectID *int64
	if in.Group != "" || in.Project != "" {
		projectIDs, projectID, err = resolveFilterScope(ctx, s.client, in.Group, in.Project)
		if err != nil {
			return nil, FilterCommentsOutput{}, fmt.Errorf("filter_comments: %w", err)
		}
	}

	req, err := newFilterCommentsRequest(in, from, to, projectIDs, projectID)
	if err != nil {
		return nil, FilterCommentsOutput{}, err
	}

	result, err := app.FilterComments(req)
	if err != nil {
		return nil, FilterCommentsOutput{}, fmt.Errorf("filter_comments: %w", err)
	}

	return nil, FilterCommentsOutput{Path: result.Path, Count: len(result.Doc.Items)}, nil
}
