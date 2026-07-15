package main

import (
	"log/slog"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rus-lan/bogoslavAnalytics/internal/gitlab"
)

// serverVersion is bogoslav-mcp's own reported MCP implementation
// version.
const serverVersion = "0.1.0"

// toolServer holds what every tool handler needs: a GitLab client --
// app's own FindMRsClient, DiscussionsClient and UserResolver interfaces
// (TZ.md section 2.4) are all satisfied by *gitlab.Client -- and the
// instance URL recorded on freshly written artifacts (TZ.md section
// 2.5).
type toolServer struct {
	client    *gitlab.Client
	gitlabURL string
}

// newServer builds the bogoslav-mcp MCP server: exactly six tools, one
// per internal/app use case (TZ.md sections 7.2, 7.3), each a thin
// wrapper -- parse input, call one app function, shape the output. No
// GitLab call, filter, cache decision, or boundary predicate is decided
// in this package; every one of those already lives in
// internal/app and the packages it composes. Tool names are
// snake_case (TZ.md section 7.2), intentionally distinct from
// bogoslav-cli's kebab-case commands.
func newServer(client *gitlab.Client, gitlabURL string, logger *slog.Logger) *mcp.Server {
	s := &toolServer{client: client, gitlabURL: gitlabURL}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "bogoslav-mcp",
		Title:   "bogoslav-mcp",
		Version: serverVersion,
	}, &mcp.ServerOptions{
		Logger: logger,
		Instructions: "GitLab review-activity analytics: find, fetch, label, filter, and " +
			"summarize merge request comments. Pipeline: find_mrs -> get_comments -> " +
			"get_classify_batch -> save_labels -> filter_comments -> get_stats. Any step " +
			"also runs on its own via from_artifact. get_classify_batch never calls a " +
			"model: it hands back the batch, taxonomy, JSON Schema and prompt for the " +
			"calling agent to label; save_labels validates the result and writes nothing " +
			"on a rejected labeling.",
	})

	mcp.AddTool(server, &mcp.Tool{
		Name: "find_mrs",
		Description: "Find merge requests where a user left more than N comments in a date " +
			"range, or -- in point mode (project + mr) -- return exactly one merge request " +
			"with no candidate search of any kind. Writes (or reuses a cached) mr_list artifact.",
	}, s.findMRs)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_comments",
		Description: "Fetch the comments a user left across a set of merge requests -- from " +
			"an existing mr_list artifact or an explicit list. Writes (or reuses a cached) " +
			"comment_list artifact.",
	}, s.getComments)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_classify_batch",
		Description: "Hand back a batch of comments, the taxonomy, the labeling result's " +
			"JSON Schema, and a rendered prompt for the calling agent to label. Never calls " +
			"a model itself. If an unchanged batch already has a matching labeled_comments " +
			"artifact for the same model and taxonomy version, reports cached=true instead.",
	}, s.getClassifyBatch)

	mcp.AddTool(server, &mcp.Tool{
		Name: "save_labels",
		Description: "Validate a labeling result -- produced by the calling agent, never by " +
			"this tool -- against the comment_list batch and the taxonomy, and only on " +
			"success write a labeled_comments artifact with the mandatory classifier " +
			"provenance block. A labeling that fails validation (a label outside the " +
			"taxonomy, an extra, missing, or duplicate note_id) writes no file in any " +
			"format and returns every violation.",
	}, s.saveLabels)

	mcp.AddTool(server, &mcp.Tool{
		Name: "filter_comments",
		Description: "Narrow a labeled_comments artifact down to a set of labels, with " +
			"optional further narrowing by date range and by group or project. Writes a " +
			"filtered_comments artifact; never consults a cache before running.",
	}, s.filterComments)

	mcp.AddTool(server, &mcp.Tool{
		Name: "get_stats",
		Description: "Aggregate the items of any one artifact (mr_list, comment_list, " +
			"labeled_comments, or filtered_comments): total count and, where applicable, " +
			"breakdowns by merge request, by label, and by day. Never calls GitLab.",
	}, s.getStats)

	return server
}
