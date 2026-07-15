package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// GetCommentsInput is the get_comments tool's input: the MCP mirror of
// bogoslav-cli's get-comments command and app.GetCommentsRequest (TZ.md
// sections 7.2, 7.3).
type GetCommentsInput struct {
	User         string           `json:"user" jsonschema:"GitLab username or numeric user id"`
	From         string           `json:"from" jsonschema:"start of the date range, inclusive, YYYY-MM-DD"`
	To           string           `json:"to" jsonschema:"end of the date range, inclusive, YYYY-MM-DD"`
	FromArtifact string           `json:"from_artifact,omitempty" jsonschema:"path to an existing mr_list artifact (json or yaml only) whose merge requests to fetch comments for; mutually exclusive with mrs"`
	MRs          []artifact.MRRef `json:"mrs,omitempty" jsonschema:"explicit merge request list (project_id, mr_iid pairs) to fetch comments for; mutually exclusive with from_artifact"`

	ArtifactsDir    string `json:"artifacts_dir,omitempty" jsonschema:"directory the comment_list artifact is written under, and where a matching artifact is looked up as a cache before calling GitLab (default \"artifacts\")"`
	Format          string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml). Artifacts double as the cache: json and yaml round-trip and are looked up before calling GitLab. text and html are write-only -- neither is readable back, neither can ever be chained via from_artifact into a later tool, and neither is ever a cache hit"`
	Refresh         bool   `json:"refresh,omitempty" jsonschema:"bypass the cache and always call GitLab, even if a fresh cached comment_list artifact already exists"`
	CacheTTLSeconds int64  `json:"cache_ttl_seconds,omitempty" jsonschema:"how long, in seconds, a cached comment_list artifact stays fresh before this tool calls GitLab again (default 86400, 24h)"`
}

// GetCommentsOutput is the get_comments tool's output.
type GetCommentsOutput struct {
	Path     string `json:"path" jsonschema:"path to the written (or, on a cache hit, already-existing) comment_list artifact"`
	CacheHit bool   `json:"cache_hit" jsonschema:"true when this result came from an existing artifact without calling GitLab"`
	Count    int    `json:"count" jsonschema:"number of comments in the result"`
}

// newGetCommentsRequest converts in and an already-resolved userID into
// an app.GetCommentsRequest. It makes no GitLab call itself: userID
// resolution (TZ.md section 5.0) is getComments's job, so this mapping
// stays pure and testable on its own, mirroring bogoslav-cli's
// newGetCommentsRequest.
func newGetCommentsRequest(in GetCommentsInput, gitlabURL string, userID int64) (app.GetCommentsRequest, error) {
	from, err := domain.ParseDate(in.From)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("from: %w", err)
	}
	to, err := domain.ParseDate(in.To)
	if err != nil {
		return app.GetCommentsRequest{}, fmt.Errorf("to: %w", err)
	}
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.GetCommentsRequest{}, err
	}

	return app.GetCommentsRequest{
		GitlabURL:    gitlabURL,
		UserID:       userID,
		From:         from,
		To:           to,
		FromArtifact: in.FromArtifact,
		MRs:          in.MRs,
		Dir:          in.ArtifactsDir,
		Format:       format,
		Cache:        cacheOptions(in.Refresh, in.CacheTTLSeconds),
	}, nil
}

// getComments is the get_comments tool handler: resolve user, build the
// request, and call app.GetComments (TZ.md section 7.3: one function of
// apps/internal/app per tool).
func (s *toolServer) getComments(ctx context.Context, _ *mcp.CallToolRequest, in GetCommentsInput) (*mcp.CallToolResult, GetCommentsOutput, error) {
	userID, err := app.ResolveUser(ctx, s.client, in.User)
	if err != nil {
		return nil, GetCommentsOutput{}, fmt.Errorf("get_comments: %w", err)
	}

	req, err := newGetCommentsRequest(in, s.gitlabURL, userID)
	if err != nil {
		return nil, GetCommentsOutput{}, err
	}

	result, err := app.GetComments(ctx, s.client, req)
	if err != nil {
		return nil, GetCommentsOutput{}, fmt.Errorf("get_comments: %w", err)
	}

	return nil, GetCommentsOutput{
		Path:     result.Path,
		CacheHit: result.CacheHit,
		Count:    len(result.Doc.Items),
	}, nil
}
