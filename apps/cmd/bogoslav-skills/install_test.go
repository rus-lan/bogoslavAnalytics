package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInstall runs `bogoslav-skills install <args...>` against a fresh
// root command, returning its stdout, stderr, and error.
func runInstall(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(append([]string{"install"}, args...))
	err = root.Execute()
	return out.String(), errOut.String(), err
}

func TestInstall_targetAndAllAreMutuallyExclusive(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := runInstall(t, "--target", "claude", "--all", "--project-dir", tmp)
	if err == nil {
		t.Fatal("expected an error when both --target and --all are set")
	}
}

func TestInstall_requiresTargetOrAll(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := runInstall(t, "--project-dir", tmp)
	if err == nil {
		t.Fatal("expected an error when neither --target nor --all is set")
	}
}

func TestInstall_rejectsRooCode(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := runInstall(t, "--target", "roo", "--project-dir", tmp)
	if err == nil {
		t.Fatal("expected an error for --target roo")
	}
	if !strings.Contains(err.Error(), "roo") {
		t.Errorf("error %v does not mention the rejected target", err)
	}
}

// TestInstall_claudeWritesSkillAndFamilyAConfig covers a single-target
// install end to end: both skill directories and the family A config.
func TestInstall_claudeWritesSkillAndFamilyAConfig(t *testing.T) {
	tmp := t.TempDir()

	_, _, err := runInstall(t, "--target", "claude", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp")
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	assertFileContains(t, filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"), "name: bogoslav")
	assertFileContains(t, filepath.Join(tmp, ".agents", "skills", serverName, "SKILL.md"), "name: bogoslav")

	config := readFile(t, filepath.Join(tmp, ".mcp.json"))
	assertSameJSON(t, config, []byte(`{"mcpServers":{"bogoslav":{"command":"/path/to/bogoslav-mcp","args":[],"env":{}}}}`))
}

// TestInstall_aiderWritesConventionsAndNoMCPConfig is TZ.md section
// 9.3.5: aider gets CONVENTIONS.md and nothing else -- no skill files,
// no MCP registration of any kind.
func TestInstall_aiderWritesConventionsAndNoMCPConfig(t *testing.T) {
	tmp := t.TempDir()

	stdout, stderr, err := runInstall(t, "--target", "aider", "--project-dir", tmp)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	assertFileContains(t, filepath.Join(tmp, "CONVENTIONS.md"), "find-mrs")
	if !strings.Contains(stdout, "CONVENTIONS.md") {
		t.Errorf("stdout does not report writing CONVENTIONS.md: %q", stdout)
	}
	if !strings.Contains(stderr, "aider --read CONVENTIONS.md") {
		t.Errorf("stderr does not explain how to use CONVENTIONS.md with aider: %q", stderr)
	}

	for _, p := range []string{
		filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".agents", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".mcp.json"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("aider install must not create %q, but it exists", p)
		}
	}
}

// TestInstall_allCoversEveryLiveTargetAndAider drives install --all and
// checks every one of the six targets left its mark, using HOME so
// cline's global config lands inside the test's temp directory.
func TestInstall_allCoversEveryLiveTargetAndAider(t *testing.T) {
	tmp := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	_, _, err := runInstall(t, "--all", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp")
	if err != nil {
		t.Fatalf("install --all: %v", err)
	}

	for _, p := range []string{
		filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".agents", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".mcp.json"),           // claude
		filepath.Join(tmp, "opencode.json"),       // opencode
		filepath.Join(tmp, "kilo.jsonc"),          // kilo
		filepath.Join(tmp, ".cursor", "mcp.json"), // cursor
		filepath.Join(fakeHome, ".cline", "mcp.json"),
		filepath.Join(tmp, "CONVENTIONS.md"), // aider
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("install --all did not create %q: %v", p, err)
		}
	}

	for _, p := range []string{
		filepath.Join(tmp, ".mcp.json"),
		filepath.Join(tmp, "opencode.json"),
		filepath.Join(tmp, "kilo.jsonc"),
		filepath.Join(tmp, ".cursor", "mcp.json"),
		filepath.Join(fakeHome, ".cline", "mcp.json"),
	} {
		assertFileContains(t, p, "bogoslav")
	}
}

// TestInstall_isIdempotent runs install --all twice and checks every
// written file is byte-identical between the two runs (TZ.md section
// 9.3: "installing twice must be idempotent").
func TestInstall_isIdempotent(t *testing.T) {
	tmp := t.TempDir()
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	paths := []string{
		filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".mcp.json"),
		filepath.Join(tmp, "opencode.json"),
		filepath.Join(tmp, "kilo.jsonc"),
		filepath.Join(tmp, ".cursor", "mcp.json"),
		filepath.Join(fakeHome, ".cline", "mcp.json"),
		filepath.Join(tmp, "CONVENTIONS.md"),
	}

	if _, _, err := runInstall(t, "--all", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp"); err != nil {
		t.Fatalf("first install --all: %v", err)
	}
	first := make(map[string][]byte, len(paths))
	for _, p := range paths {
		first[p] = readFile(t, p)
	}

	stdout, _, err := runInstall(t, "--all", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp")
	if err != nil {
		t.Fatalf("second install --all: %v", err)
	}
	for _, p := range paths {
		second := readFile(t, p)
		if !bytes.Equal(first[p], second) {
			t.Errorf("%q changed between installs:\nfirst:\n%s\nsecond:\n%s", p, first[p], second)
		}
	}
	if strings.Contains(stdout, "create ") {
		t.Errorf("second install reported a create, want only unchanged/update: %q", stdout)
	}
}

// TestInstall_symlinkedConfigIsReportedInOutput covers the symlink
// finding end to end: a project that ships its .mcp.json as a symlink
// pointing outside the project directory still gets it merged, but
// install's own stdout names the real, out-of-tree file that was
// actually written -- so the user is told, never surprised.
func TestInstall_symlinkedConfigIsReportedInOutput(t *testing.T) {
	tmp := t.TempDir()
	outside := t.TempDir()

	realPath := filepath.Join(outside, "real-mcp.json")
	if err := os.WriteFile(realPath, []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatalf("write real config: %v", err)
	}
	wantReal, err := filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if err := os.Symlink(realPath, filepath.Join(tmp, ".mcp.json")); err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	stdout, _, err := runInstall(t, "--target", "claude", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if !strings.Contains(stdout, wantReal) {
		t.Errorf("stdout does not name the symlink's real target %q:\n%s", wantReal, stdout)
	}

	assertFileContains(t, realPath, "bogoslav")
}

func TestInstall_dryRunWritesNothing(t *testing.T) {
	tmp := t.TempDir()

	stdout, _, err := runInstall(t, "--target", "claude", "--project-dir", tmp, "--mcp-command", "/path/to/bogoslav-mcp", "--dry-run")
	if err != nil {
		t.Fatalf("install --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "create") {
		t.Errorf("dry-run stdout does not report what would be created: %q", stdout)
	}

	for _, p := range []string{
		filepath.Join(tmp, ".claude", "skills", serverName, "SKILL.md"),
		filepath.Join(tmp, ".mcp.json"),
	} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("--dry-run created %q", p)
		}
	}
}

func assertFileContains(t *testing.T, path, substr string) {
	t.Helper()
	content := readFile(t, path)
	if !bytes.Contains(content, []byte(substr)) {
		t.Errorf("%q does not contain %q; content:\n%s", path, substr, content)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return content
}
