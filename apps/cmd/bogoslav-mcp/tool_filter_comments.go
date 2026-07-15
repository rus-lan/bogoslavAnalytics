package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// FilterCommentsInput is the filter_comments tool's input: the MCP
// mirror of bogoslav-cli's filter-comments command and
// app.FilterCommentsRequest (TZ.md sections 7.2, 7.3).
type FilterCommentsInput struct {
	FromArtifact string   `json:"from_artifact" jsonschema:"path to the labeled_comments artifact (json or yaml only) to filter"`
	Labels       []string `json:"labels" jsonschema:"taxonomy labels to keep; at least one is required"`
	From         string   `json:"from,omitempty" jsonschema:"start of an additional date filter, inclusive, YYYY-MM-DD; requires to"`
	To           string   `json:"to,omitempty" jsonschema:"end of an additional date filter, inclusive, YYYY-MM-DD; requires from"`
	Group        string   `json:"group,omitempty" jsonschema:"keep only comments on merge requests in this group's projects, including subgroups (numeric id or path); resolved to project ids with one GitLab call"`
	Project      string   `json:"project,omitempty" jsonschema:"keep only comments on merge requests in this single project (numeric id or path); resolved with one GitLab call if it is a namespaced path"`

	ArtifactsDir string `json:"artifacts_dir,omitempty" jsonschema:"directory the filtered_comments artifact is written under (default \"artifacts\")"`
	Format       string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml); this tool never consults a cache before running -- it always reads from_artifact and reprocesses it"`
}

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
func newFilterCommentsRequest(in FilterCommentsInput, from, to *domain.Date, projectIDs []int64, projectID *int64) (app.FilterCommentsRequest, error) {
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
func (s *toolServer) filterComments(ctx context.Context, _ *mcp.CallToolRequest, in FilterCommentsInput) (*mcp.CallToolResult, FilterCommentsOutput, error) {
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
