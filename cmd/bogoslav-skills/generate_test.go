package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestGenerate_writesSkillFilesOnlyNeverTouchesAnyToolConfig covers the
// generate-only mode's whole point: it writes SKILL.md and stops there.
func TestGenerate_writesSkillFilesOnlyNeverTouchesAnyToolConfig(t *testing.T) {
	tmp := t.TempDir()

	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"generate", "--project-dir", tmp})
	if err := root.Execute(); err != nil {
		t.Fatalf("generate: %v", err)
	}

	assertFileContains(t, filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"), "name: bogoslav")
	assertFileContains(t, filepath.Join(tmp, ".agents", "skills", serverName, "SKILL.md"), "name: bogoslav")

	for _, p := range []string{
		".mcp.json", "opencode.json", "kilo.jsonc",
		filepath.Join(".cursor", "mcp.json"), "CONVENTIONS.md",
	} {
		if _, err := os.Stat(filepath.Join(tmp, p)); !os.IsNotExist(err) {
			t.Errorf("generate must not create %q, but it exists", p)
		}
	}
}

func TestGenerate_isIdempotent(t *testing.T) {
	tmp := t.TempDir()

	run := func() string {
		root := newRootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetArgs([]string{"generate", "--project-dir", tmp})
		if err := root.Execute(); err != nil {
			t.Fatalf("generate: %v", err)
		}
		return out.String()
	}

	run()
	first := readFile(t, filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"))
	second := run()
	after := readFile(t, filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"))

	if !bytes.Equal(first, after) {
		t.Fatalf("SKILL.md changed on the second generate run")
	}
	if !bytes.Contains([]byte(second), []byte("unchanged")) {
		t.Errorf("second run's report does not say unchanged: %q", second)
	}
}
