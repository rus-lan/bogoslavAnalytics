package main

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// testLogger is a discard slog.Logger, so tests never spam stderr.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestServer builds a server with a GitLab client that is never
// actually dialed (constructing *gitlab.Client does no I/O): fine for
// every test in this file, none of which exercises a tool that talks to
// GitLab.
func newTestServer() *mcp.Server {
	client := gitlab.NewClient("http://gitlab.invalid", "test-token")
	return newServer(client, "http://gitlab.invalid", testLogger())
}

// wantToolNames is the exact tool set TZ.md section 7.2 lists, one per
// internal/app use case (TZ.md section 7.3): a rename of any one of
// these must break this test.
var wantToolNames = []string{
	"find_mrs",
	"get_comments",
	"get_classify_batch",
	"save_labels",
	"filter_comments",
	"get_stats",
}

// TestNewServer_registersExpectedToolNames is the acceptance check for
// TZ.md section 7.2: exactly six tools, named exactly as the tool table
// lists them. It connects a real mcp.Client over an in-memory transport
// pair and calls tools/list, rather than inspecting the server's
// internals, so it exercises the exact same registration path a real
// stdio client would see.
func TestNewServer_registersExpectedToolNames(t *testing.T) {
	ctx := context.Background()
	server := newTestServer()

	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "bogoslav-mcp-test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}

	var got []string
	for _, tool := range result.Tools {
		got = append(got, tool.Name)
	}
	slices.Sort(got)

	want := slices.Clone(wantToolNames)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Errorf("registered tool names = %v, want %v", got, want)
	}
}
