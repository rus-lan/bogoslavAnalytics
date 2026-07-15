package clitree

import "testing"

// TestNewRootCmd_hasExactlySixExpectedCommands proves the command tree
// has exactly the six commands TZ.md section 7.2 names, with the exact
// names this task fixes (find-mrs, get-comments, get-classify-batch,
// save-labels, filter-comments, get-stats): bogoslav-skills (a later
// wave) generates SKILL.md by walking this tree, so a rename here must
// break this test.
func TestNewRootCmd_hasExactlySixExpectedCommands(t *testing.T) {
	want := []string{
		"find-mrs",
		"get-comments",
		"get-classify-batch",
		"save-labels",
		"filter-comments",
		"get-stats",
	}

	cmds := NewRootCmd().Commands()
	if len(cmds) != len(want) {
		names := make([]string, len(cmds))
		for i, c := range cmds {
			names[i] = c.Name()
		}
		t.Fatalf("NewRootCmd().Commands() = %v, want exactly %v", names, want)
	}

	got := make(map[string]bool, len(cmds))
	for _, c := range cmds {
		got[c.Name()] = true
	}
	for _, name := range want {
		if !got[name] {
			t.Errorf("NewRootCmd() is missing command %q", name)
		}
	}
}
