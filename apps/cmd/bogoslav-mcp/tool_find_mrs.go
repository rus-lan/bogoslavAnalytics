package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// FindMRsInput is the find_mrs tool's input: the MCP mirror of
// bogoslav-cli's find-mrs command and app.FindMRsRequest (TZ.md sections
// 7.2, 7.3).
type FindMRsInput struct {
	User string `json:"user" jsonschema:"GitLab username or numeric user id"`
	From string `json:"from" jsonschema:"start of the date range, inclusive, YYYY-MM-DD"`
	To   string `json:"to" jsonschema:"end of the date range, inclusive, YYYY-MM-DD"`
	// MoreThan is the N threshold: the boundary is honestly STRICT, not
	// >=. See the field's own description for the exact contract.
	MoreThan int    `json:"more_than" jsonschema:"return only merge requests where user left STRICTLY more than this many comments in [from, to]; a merge request with exactly this many comments is NOT returned, only one with more_than + 1 or more"`
	Group    string `json:"group,omitempty" jsonschema:"restrict the search to this group's projects, including subgroups (numeric id or path)"`
	Project  string `json:"project,omitempty" jsonschema:"restrict the search to this single project (numeric id or path); required together with mr"`
	MR       int64  `json:"mr,omitempty" jsonschema:"point mode: return exactly this merge request iid, with no candidate search of any kind (no events, no bruteforce, no autoselector); requires project; 0 or omitted means no point mode"`
	Strict   bool   `json:"strict,omitempty" jsonschema:"force the bruteforce search strategy, skipping the events strategy and its smoke test"`

	ArtifactsDir    string `json:"artifacts_dir,omitempty" jsonschema:"directory the mr_list artifact is written under, and where a matching artifact is looked up as a cache before calling GitLab (default \"artifacts\")"`
	Format          string `json:"format,omitempty" jsonschema:"artifact wire format: json, yaml, text, or html (default yaml). Artifacts double as the cache: json and yaml round-trip and are looked up before calling GitLab. text and html are write-only -- neither is readable back, neither can ever be chained via from_artifact into a later tool, and neither is ever a cache hit"`
	Refresh         bool   `json:"refresh,omitempty" jsonschema:"bypass the cache and always call GitLab, even if a fresh cached mr_list artifact already exists"`
	CacheTTLSeconds int64  `json:"cache_ttl_seconds,omitempty" jsonschema:"how long, in seconds, a cached mr_list artifact stays fresh before this tool calls GitLab again (default 86400, 24h)"`
}

// FindMRsOutput is the find_mrs tool's output.
type FindMRsOutput struct {
	Path     string `json:"path" jsonschema:"path to the written (or, on a cache hit, already-existing) mr_list artifact"`
	CacheHit bool   `json:"cache_hit" jsonschema:"true when this result came from an existing artifact without calling GitLab"`
	Count    int    `json:"count" jsonschema:"number of merge requests in the result"`
	Strategy string `json:"strategy" jsonschema:"which candidate search strategy actually ran, events or bruteforce; empty in point mode, where no candidate search runs at all"`
	Smoke    string `json:"smoke" jsonschema:"result of the DiscussionNote smoke test that gated the strategy choice: passed, failed, or unknown; empty in point mode"`
}

// newFindMRsRequest converts in into an app.FindMRsRequest. It makes no
// GitLab call and does not resolve user: app.FindMRs does both itself
// (TZ.md section 5.0), so this mapping stays pure and testable on its
// own, mirroring bogoslav-cli's newFindMRsRequest.
func newFindMRsRequest(in FindMRsInput, gitlabURL string) (app.FindMRsRequest, error) {
	from, err := domain.ParseDate(in.From)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("from: %w", err)
	}
	to, err := domain.ParseDate(in.To)
	if err != nil {
		return app.FindMRsRequest{}, fmt.Errorf("to: %w", err)
	}
	format, err := parseFormat(in.Format)
	if err != nil {
		return app.FindMRsRequest{}, err
	}

	req := app.FindMRsRequest{
		GitlabURL: gitlabURL,
		User:      in.User,
		From:      from,
		To:        to,
		MoreThan:  in.MoreThan,
		Group:     in.Group,
		Project:   in.Project,
		Strict:    in.Strict,
		Dir:       in.ArtifactsDir,
		Format:    format,
		Cache:     cacheOptions(in.Refresh, in.CacheTTLSeconds),
	}
	if in.MR != 0 {
		mr := in.MR
		req.MR = &mr
	}
	return req, nil
}

// findMRs is the find_mrs tool handler: build the request and call
// app.FindMRs (TZ.md section 7.3: one function of apps/internal/app per
// tool). Point mode's "mr requires project" rule (TZ.md sections 1.2,
// 7.2) is enforced by app.FindMRs itself, not here.
func (s *toolServer) findMRs(ctx context.Context, _ *mcp.CallToolRequest, in FindMRsInput) (*mcp.CallToolResult, FindMRsOutput, error) {
	req, err := newFindMRsRequest(in, s.gitlabURL)
	if err != nil {
		return nil, FindMRsOutput{}, err
	}

	result, err := app.FindMRs(ctx, s.client, req)
	if err != nil {
		return nil, FindMRsOutput{}, fmt.Errorf("find_mrs: %w", err)
	}

	return nil, FindMRsOutput{
		Path:     result.Path,
		CacheHit: result.CacheHit,
		Count:    len(result.Doc.Items),
		Strategy: string(result.Doc.Query.Strategy),
		Smoke:    string(result.Doc.Query.Smoke),
	}, nil
}
