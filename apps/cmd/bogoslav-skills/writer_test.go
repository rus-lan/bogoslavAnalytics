package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileTracked_createsMissingFileAndParentDirectories(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a", "b", "SKILL.md")

	c, err := writeFileTracked(path, []byte("content"), false)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeCreated {
		t.Errorf("kind = %v, want changeCreated", c.kind)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "content" {
		t.Errorf("content = %q, want %q", got, "content")
	}
}

func TestWriteFileTracked_reportsUnchangedAndWritesNothingWhenContentMatches(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "SKILL.md")
	writeTestFile(t, path, "content")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	c, err := writeFileTracked(path, []byte("content"), false)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeUnchanged {
		t.Errorf("kind = %v, want changeUnchanged", c.kind)
	}
	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.ModTime() != after.ModTime() {
		t.Error("file was rewritten even though content did not change")
	}
}

func TestWriteFileTracked_updatesWhenContentDiffers(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "SKILL.md")
	writeTestFile(t, path, "old content")

	c, err := writeFileTracked(path, []byte("new content"), false)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeUpdated {
		t.Errorf("kind = %v, want changeUpdated", c.kind)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new content" {
		t.Errorf("content = %q, want %q", got, "new content")
	}
}

func TestWriteFileTracked_dryRunWritesNothing(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a", "SKILL.md")

	c, err := writeFileTracked(path, []byte("content"), true)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeCreated {
		t.Errorf("kind = %v, want changeCreated (reported, not applied)", c.kind)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote a file at %q", path)
	}
}

// TestWriteSkillFiles_writesBothClaudeAndAgentsDirectories is TZ.md
// section 9.1: one skill artifact, written to BOTH .claude/skills/ and
// .agents/skills/, byte-identical.
func TestWriteSkillFiles_writesBothClaudeAndAgentsDirectories(t *testing.T) {
	tmp := t.TempDir()

	changes, err := writeSkillFiles(tmp, "skill content", false)
	if err != nil {
		t.Fatalf("writeSkillFiles: %v", err)
	}
	if len(changes) != 2 {
		t.Fatalf("got %d changes, want 2", len(changes))
	}

	claudePath := filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md")
	agentsPath := filepath.Join(tmp, ".agents", "skills", serverName, "SKILL.md")

	claude, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read .claude copy: %v", err)
	}
	agents, err := os.ReadFile(agentsPath)
	if err != nil {
		t.Fatalf("read .agents copy: %v", err)
	}
	if string(claude) != "skill content" || string(agents) != "skill content" {
		t.Errorf("claude=%q agents=%q, want both %q", claude, agents, "skill content")
	}
}

func TestMergeConfigFile_readsCreatesAndReportsChange(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".mcp.json")

	entry := mustEntryJSON(t, familyA, serverDescriptor{Command: "/x", Args: []string{}, Env: map[string]string{}})

	c, err := mergeConfigFile(path, "mcpServers", "bogoslav", entry, false)
	if err != nil {
		t.Fatalf("mergeConfigFile: %v", err)
	}
	if c.kind != changeCreated {
		t.Errorf("kind = %v, want changeCreated", c.kind)
	}

	c2, err := mergeConfigFile(path, "mcpServers", "bogoslav", entry, false)
	if err != nil {
		t.Fatalf("mergeConfigFile (second run): %v", err)
	}
	if c2.kind != changeUnchanged {
		t.Errorf("second run kind = %v, want changeUnchanged (idempotent)", c2.kind)
	}
}
