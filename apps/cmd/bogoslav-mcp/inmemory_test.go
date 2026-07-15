package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
)

// TestSaveLabels_endToEndOverInMemoryTransport is an end-to-end check
// that the save_labels tool works over a real MCP session (not just as
// a direct Go call, as save_labels_test.go exercises): input arguments
// travel through JSON marshaling/unmarshaling and schema validation
// exactly as a real stdio client's call would, using the SDK's
// InMemoryTransport pair instead of spawning a subprocess.
func TestSaveLabels_endToEndOverInMemoryTransport(t *testing.T) {
	dir := t.TempDir()
	commentListPath := writeFixtureCommentList(t, dir)

	ctx := context.Background()
	client := gitlab.NewClient("http://gitlab.invalid", "test-token")
	server := newServer(client, "http://gitlab.invalid", testLogger())

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "bogoslav-mcp-test-client", Version: "test"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "save_labels",
		Arguments: map[string]any{
			"from_artifact": commentListPath,
			"labels":        []map[string]any{{"note_id": 1, "label": "bug"}},
			"tool":          "test-tool",
			"model":         "test-model",
			"artifacts_dir": dir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(save_labels) error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool(save_labels) result.IsError = true, content = %+v", result.Content)
	}

	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal StructuredContent error = %v", err)
	}
	var out SaveLabelsOutput
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal StructuredContent error = %v", err)
	}
	if out.Path == "" {
		t.Error("Path = \"\", want a written labeled_comments artifact path")
	}
	if out.Count != 1 {
		t.Errorf("Count = %d, want 1", out.Count)
	}
}

// TestSaveLabels_endToEndOverInMemoryTransport_rejectsBadLabel confirms
// the protocol round trip also carries a rejected labeling's error back
// to the caller as a tool error, not a silently empty success.
func TestSaveLabels_endToEndOverInMemoryTransport_rejectsBadLabel(t *testing.T) {
	dir := t.TempDir()
	commentListPath := writeFixtureCommentList(t, dir)

	ctx := context.Background()
	client := gitlab.NewClient("http://gitlab.invalid", "test-token")
	server := newServer(client, "http://gitlab.invalid", testLogger())

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()

	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "bogoslav-mcp-test-client", Version: "test"}, nil)
	clientSession, err := mcpClient.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "save_labels",
		Arguments: map[string]any{
			"from_artifact": commentListPath,
			"labels":        []map[string]any{{"note_id": 1, "label": "not-a-real-label"}},
			"tool":          "test-tool",
			"model":         "test-model",
			"artifacts_dir": dir,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(save_labels) error = %v", err)
	}
	if !result.IsError {
		t.Fatal("CallTool(save_labels) result.IsError = false, want true for an out-of-taxonomy label")
	}
}
