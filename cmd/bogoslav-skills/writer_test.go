package main

import (
	"os"
	"path/filepath"
	"strings"
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

// TestWriteFileTracked_normalFileReportsNoSymlink is the non-symlink
// control for the symlink-following tests below: a plain file's change
// never carries a real target or an "(symlink -> ...)" note.
func TestWriteFileTracked_normalFileReportsNoSymlink(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "SKILL.md")
	writeTestFile(t, path, "old content")

	c, err := writeFileTracked(path, []byte("new content"), false)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.real != "" {
		t.Errorf("real = %q for a plain file, want empty", c.real)
	}
	if strings.Contains(c.String(), "symlink") {
		t.Errorf("String() mentions a symlink for a plain file: %q", c.String())
	}
}

// TestWriteFileTracked_symlinkedConfigIsFollowedAndReported is the fix for
// the symlink finding: a config that is a symlink pointing outside the
// directory tree install is running against is still merged (see
// writeFileTracked's doc comment on why refusing outright would break
// legitimate dotfile-manager setups), but the write must land at, and be
// reported as, the symlink's real target -- never a silent surprise.
// Removing the symlink resolution (c.real staying "") makes this test
// fail.
func TestWriteFileTracked_symlinkedConfigIsFollowedAndReported(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()

	realPath := filepath.Join(outsideDir, "real-config.json")
	writeTestFile(t, realPath, "old content")
	wantReal, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", realPath, err)
	}

	linkPath := filepath.Join(projectDir, ".mcp.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	c, err := writeFileTracked(linkPath, []byte("new content"), false)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeUpdated {
		t.Errorf("kind = %v, want changeUpdated", c.kind)
	}
	if c.real != wantReal {
		t.Errorf("real = %q, want %q (the symlink's actual target)", c.real, wantReal)
	}
	if !strings.Contains(c.String(), wantReal) {
		t.Errorf("String() = %q, does not mention the real target %q", c.String(), wantReal)
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read real target: %v", err)
	}
	if string(got) != "new content" {
		t.Errorf("real target content = %q, want %q", got, "new content")
	}

	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("lstat link: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Error("writeFileTracked replaced the symlink itself instead of writing through it")
	}
}

// TestWriteFileTracked_dryRunSymlinkedConfigWritesNothingAnywhere checks
// --dry-run's "writes nothing" guarantee still holds through a symlink:
// neither the link nor its real, out-of-tree target is touched, even
// though the reported change still names the real target.
func TestWriteFileTracked_dryRunSymlinkedConfigWritesNothingAnywhere(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()

	realPath := filepath.Join(outsideDir, "real-config.json")
	writeTestFile(t, realPath, "old content")
	wantReal, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q): %v", realPath, err)
	}

	linkPath := filepath.Join(projectDir, ".mcp.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	c, err := writeFileTracked(linkPath, []byte("new content"), true)
	if err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}
	if c.kind != changeUpdated {
		t.Errorf("kind = %v, want changeUpdated (reported, not applied)", c.kind)
	}
	if c.real != wantReal {
		t.Errorf("real = %q, want %q", c.real, wantReal)
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read real target: %v", err)
	}
	if string(got) != "old content" {
		t.Errorf("--dry-run wrote through the symlink; real target content = %q, want unchanged %q", got, "old content")
	}
}

// TestWriteFileTracked_preservesExistingFilePermissions guards against a
// regression in the symlink fix: os.WriteFile's mode argument only
// applies when it creates a file, so merging into an existing, more
// restrictively-permissioned config must leave its permissions alone.
func TestWriteFileTracked_preservesExistingFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".mcp.json")
	writeTestFile(t, path, "old content")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	if _, err := writeFileTracked(path, []byte("new content"), false); err != nil {
		t.Fatalf("writeFileTracked: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %o, want %o (preserved from before the merge)", info.Mode().Perm(), 0o600)
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

// TestMergeConfigFile_symlinkedConfigIsFollowedAndReported is the exact
// scenario the symlink finding describes: a project ships its .mcp.json
// as a symlink pointing outside the project directory. install still
// merges into it, but the resulting change always names the real file
// that got modified, so the write is never a silent surprise.
func TestMergeConfigFile_symlinkedConfigIsFollowedAndReported(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()

	realPath := filepath.Join(outsideDir, "real-mcp.json")
	writeTestFile(t, realPath, `{"mcpServers":{}}`)
	wantReal, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	linkPath := filepath.Join(projectDir, ".mcp.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	entry := mustEntryJSON(t, familyA, serverDescriptor{Command: "/x", Args: []string{}, Env: map[string]string{}})

	c, err := mergeConfigFile(linkPath, "mcpServers", "bogoslav", entry, false)
	if err != nil {
		t.Fatalf("mergeConfigFile: %v", err)
	}
	if c.real != wantReal {
		t.Errorf("real = %q, want %q", c.real, wantReal)
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("read real target: %v", err)
	}
	assertSameJSON(t, got, []byte(`{"mcpServers":{"bogoslav":{"command":"/x","args":[],"env":{}}}}`))
}
