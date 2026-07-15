package clitree_test

import (
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/clitree"
)

// TestNewRootCmd_importableFromADifferentPackage proves clitree.NewRootCmd
// is actually importable and usable from outside the clitree package
// itself -- the whole point of extracting the cobra tree out of
// cmd/bogoslav-cli's package main, where bogoslav-skills (a later
// wave) could never have imported it (TZ.md section 9).
func TestNewRootCmd_importableFromADifferentPackage(t *testing.T) {
	want := []string{
		"find-mrs",
		"get-comments",
		"get-classify-batch",
		"save-labels",
		"filter-comments",
		"get-stats",
	}

	cmds := clitree.NewRootCmd().Commands()
	if len(cmds) != len(want) {
		names := make([]string, len(cmds))
		for i, c := range cmds {
			names[i] = c.Name()
		}
		t.Fatalf("clitree.NewRootCmd().Commands() = %v, want exactly %v", names, want)
	}

	got := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		got[c.Name()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("clitree.NewRootCmd() is missing command %q", name)
		}
	}
}
