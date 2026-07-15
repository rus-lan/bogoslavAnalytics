package main

import (
	"strings"
	"testing"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/clitree"
)

func TestRenderConventionsMarkdown_hasNoFrontmatterAndListsEveryCommand(t *testing.T) {
	out := renderConventionsMarkdown(clitree.NewRootCmd())

	if strings.HasPrefix(out, "---\n") {
		t.Error("CONVENTIONS.md must not carry Agent Skills frontmatter: aider does not read skills at all")
	}
	for _, name := range []string{
		"find-mrs", "get-comments", "get-classify-batch",
		"save-labels", "filter-comments", "get-stats",
	} {
		if !strings.Contains(out, name) {
			t.Errorf("CONVENTIONS.md is missing command %q", name)
		}
	}
}

func TestRenderConventionsMarkdown_regeneratingIsZeroDiff(t *testing.T) {
	first := renderConventionsMarkdown(clitree.NewRootCmd())
	second := renderConventionsMarkdown(clitree.NewRootCmd())
	if first != second {
		t.Fatal("regenerating CONVENTIONS.md produced a diff")
	}
}
