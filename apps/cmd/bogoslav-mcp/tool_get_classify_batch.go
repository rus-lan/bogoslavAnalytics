package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/mcptool"
)

// newGetClassifyBatchRequest converts in into an app.GetClassifyBatchRequest.
func newGetClassifyBatchRequest(in mcptool.GetClassifyBatchInput) app.GetClassifyBatchRequest {
	return app.GetClassifyBatchRequest{
		CommentListPath: in.FromArtifact,
		Model:           in.Model,
		Taxonomy:        in.Taxonomy,
		Dir:             in.ArtifactsDir,
	}
}

// getClassifyBatch is the get_classify_batch tool handler: build the
// request and call app.GetClassifyBatch (TZ.md section 7.3: one function
// of apps/internal/app per tool). It makes no LLM call.
func (s *toolServer) getClassifyBatch(_ context.Context, _ *mcp.CallToolRequest, in mcptool.GetClassifyBatchInput) (*mcp.CallToolResult, any, error) {
	result, err := app.GetClassifyBatch(newGetClassifyBatchRequest(in))
	if err != nil {
		return nil, nil, fmt.Errorf("get_classify_batch: %w", err)
	}

	out := mcptool.GetClassifyBatchOutput{
		Cached:       result.Cached,
		ArtifactPath: result.ArtifactPath,
		Batch:        result.Batch,
		Taxonomy:     result.Taxonomy,
		Schema:       result.Schema,
		Prompt:       result.Prompt,
	}
	return nil, out, nil
}
