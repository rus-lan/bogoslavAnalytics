package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/clitree"
)

// TestRenderSkillMarkdown_regeneratingFromUnchangedTreeIsZeroDiff is
// TZ.md section 12, acceptance criterion 14: regenerating from the same
// (unchanged) command tree must reproduce the exact same bytes.
func TestRenderSkillMarkdown_regeneratingFromUnchangedTreeIsZeroDiff(t *testing.T) {
	first := renderSkillMarkdown(clitree.NewRootCmd())
	second := renderSkillMarkdown(clitree.NewRootCmd())

	if first != second {
		t.Fatalf("regenerating produced a diff:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestRenderSkillMarkdown_matchesLiveCommandTree proves the content is
// actually generated from clitree.NewRootCmd(), not a hand-written copy:
// every one of the six real command names, and a real flag from one of
// them, must appear.
func TestRenderSkillMarkdown_matchesLiveCommandTree(t *testing.T) {
	out := renderSkillMarkdown(clitree.NewRootCmd())

	for _, name := range []string{
		"find-mrs", "get-comments", "get-classify-batch",
		"save-labels", "filter-comments", "get-stats",
	} {
		if !strings.Contains(out, "`"+name+"`") {
			t.Errorf("SKILL.md is missing command %q", name)
		}
	}
	for _, tool := range []string{
		"find_mrs", "get_comments", "get_classify_batch",
		"save_labels", "filter_comments", "get_stats",
	} {
		if !strings.Contains(out, tool) {
			t.Errorf("SKILL.md is missing MCP tool name %q", tool)
		}
	}
	if !strings.Contains(out, "--more-than") {
		t.Error("SKILL.md is missing a real find-mrs flag (--more-than)")
	}
}

// TestRenderSkillMarkdown_renamedCommandChangesOutput is TZ.md section
// 12, acceptance criterion 14's other half: a renamed command must
// change the generated output. It builds its own synthetic tree rather
// than editing apps/internal/clitree (out of this command's scope), which
// also proves renderSkillMarkdown works on any *cobra.Command tree, not
// just the one production wires up.
func TestRenderSkillMarkdown_renamedCommandChangesOutput(t *testing.T) {
	before := renderSkillMarkdown(fakeRootCmd("find-mrs"))
	after := renderSkillMarkdown(fakeRootCmd("locate-mrs"))

	if before == after {
		t.Fatal("renaming a command did not change renderSkillMarkdown's output")
	}
	if !strings.Contains(after, "`locate-mrs`") {
		t.Errorf("renamed command missing from output:\n%s", after)
	}
	if strings.Contains(after, "`find-mrs`") {
		t.Errorf("old command name survived the rename:\n%s", after)
	}
}

func TestRenderSkillMarkdown_frontmatterIsPortableCoreOnly(t *testing.T) {
	out := renderSkillMarkdown(clitree.NewRootCmd())

	front, _, ok := strings.Cut(strings.TrimPrefix(out, "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("SKILL.md has no --- frontmatter block:\n%s", out)
	}
	if !strings.Contains(front, "name: bogoslav") {
		t.Errorf("frontmatter missing name: bogoslav, got %q", front)
	}
	if !strings.Contains(front, "description:") {
		t.Errorf("frontmatter missing description, got %q", front)
	}
	// TZ.md section 9.1: Claude-specific frontmatter extensions must not
	// appear; only name and description are portable across all five
	// live targets.
	for _, key := range []string{"license:", "allowed-tools:", "metadata:"} {
		if strings.Contains(front, key) {
			t.Errorf("frontmatter has non-portable key %q: %q", key, front)
		}
	}
}

func TestMCPToolName_transformsKebabToSnake(t *testing.T) {
	cases := map[string]string{
		"find-mrs":           "find_mrs",
		"get-comments":       "get_comments",
		"get-classify-batch": "get_classify_batch",
		"save-labels":        "save_labels",
		"filter-comments":    "filter_comments",
		"get-stats":          "get_stats",
	}
	for in, want := range cases {
		if got := mcpToolName(in); got != want {
			t.Errorf("mcpToolName(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeRootCmd builds a minimal synthetic tree with one subcommand named
// firstCommandName, for tests that must prove generation without
// touching apps/internal/clitree.
func fakeRootCmd(firstCommandName string) *cobra.Command {
	root := &cobra.Command{
		Use:   "bogoslav-cli",
		Short: "test root",
		Long:  "test root long description",
	}
	sub := &cobra.Command{
		Use:   firstCommandName,
		Short: "test subcommand",
		Long:  "test subcommand long description",
	}
	sub.Flags().String("example-flag", "", "an example flag")
	root.AddCommand(sub)
	return root
}
