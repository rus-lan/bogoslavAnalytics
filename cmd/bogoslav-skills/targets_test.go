package main

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateTargetID_acceptsEveryLiveTarget(t *testing.T) {
	for _, id := range []string{"claude", "opencode", "kilo", "cline", "cursor", "aider"} {
		if err := validateTargetID(id); err != nil {
			t.Errorf("validateTargetID(%q) = %v, want nil", id, err)
		}
	}
}

// TestValidateTargetID_rejectsRooCode covers TZ.md section 9.3.4: Roo
// Code was archived read-only by its owner on 2026-05-15 and is
// deliberately not a target, under any spelling.
func TestValidateTargetID_rejectsRooCode(t *testing.T) {
	for _, id := range []string{"roo", "roo-code", "Roo Code", "roocode"} {
		err := validateTargetID(id)
		if err == nil {
			t.Errorf("validateTargetID(%q) = nil, want an error", id)
			continue
		}
		if !errors.Is(err, ErrUnknownTarget) {
			t.Errorf("validateTargetID(%q) error = %v, want it to wrap ErrUnknownTarget", id, err)
		}
	}
}

func TestValidateTargetID_rejectsGarbage(t *testing.T) {
	if err := validateTargetID("does-not-exist"); err == nil {
		t.Fatal("expected an error for an unknown target, got nil")
	}
}

func TestFindMCPTarget_aiderIsNotAnMCPTarget(t *testing.T) {
	if _, err := findMCPTarget(aiderTargetID); err == nil {
		t.Fatal("findMCPTarget(aider) = nil error, want ErrUnknownTarget: aider has no MCP config")
	}
}

func TestFindMCPTarget_familiesMatchTZDoc(t *testing.T) {
	want := map[string]family{
		"claude":   familyA,
		"cursor":   familyA,
		"cline":    familyA,
		"opencode": familyB,
		"kilo":     familyB,
	}
	for id, wantFamily := range want {
		target, err := findMCPTarget(id)
		if err != nil {
			t.Fatalf("findMCPTarget(%q): %v", id, err)
		}
		if target.family != wantFamily {
			t.Errorf("%s family = %v, want %v", id, target.family, wantFamily)
		}
	}
}

// TestFindMCPTarget_supportsTimeoutMatchesTZDoc covers TZ.md section 9.4:
// only claude, opencode, and kilo document a per-server timeout field in
// their own config format; cline and cursor do not, and must not claim
// to.
func TestFindMCPTarget_supportsTimeoutMatchesTZDoc(t *testing.T) {
	want := map[string]bool{
		"claude":   true,
		"opencode": true,
		"kilo":     true,
		"cline":    false,
		"cursor":   false,
	}
	for id, wantSupports := range want {
		target, err := findMCPTarget(id)
		if err != nil {
			t.Fatalf("findMCPTarget(%q): %v", id, err)
		}
		if target.supportsTimeout != wantSupports {
			t.Errorf("%s supportsTimeout = %v, want %v", id, target.supportsTimeout, wantSupports)
		}
	}
}

func TestMCPTarget_resolveConfigPath(t *testing.T) {
	tmp := t.TempDir()

	claude, err := mustTarget(t, "claude").resolveConfigPath(tmp)
	if err != nil {
		t.Fatalf("claude resolveConfigPath: %v", err)
	}
	if want := filepath.Join(tmp, ".mcp.json"); claude != want {
		t.Errorf("claude config path = %q, want %q", claude, want)
	}

	cursor, err := mustTarget(t, "cursor").resolveConfigPath(tmp)
	if err != nil {
		t.Fatalf("cursor resolveConfigPath: %v", err)
	}
	if want := filepath.Join(tmp, ".cursor", "mcp.json"); cursor != want {
		t.Errorf("cursor config path = %q, want %q", cursor, want)
	}
}

// TestMCPTarget_clineIgnoresProjectDir covers TZ.md section 9.2's note
// that cline's config is a global file, not a project one: two different
// project directories must resolve to the same path.
func TestMCPTarget_clineIgnoresProjectDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cline := mustTarget(t, "cline")
	a, err := cline.resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	b, err := cline.resolveConfigPath(t.TempDir())
	if err != nil {
		t.Fatalf("resolveConfigPath: %v", err)
	}
	if a != b {
		t.Errorf("cline config path depends on project dir: %q vs %q", a, b)
	}
	if !strings.HasSuffix(a, filepath.Join(".cline", "mcp.json")) {
		t.Errorf("cline config path = %q, want a path ending in .cline/mcp.json", a)
	}
}

func TestFirstExistingOrDefault_prefersExistingFile(t *testing.T) {
	tmp := t.TempDir()
	writeTestFile(t, filepath.Join(tmp, "opencode.jsonc"), "{}")

	got := firstExistingOrDefault(tmp, "opencode.json", "opencode.jsonc")
	if want := filepath.Join(tmp, "opencode.jsonc"); got != want {
		t.Errorf("firstExistingOrDefault = %q, want %q", got, want)
	}
}

func TestFirstExistingOrDefault_defaultsToFirstCandidateWhenNoneExist(t *testing.T) {
	tmp := t.TempDir()

	got := firstExistingOrDefault(tmp, "opencode.json", "opencode.jsonc")
	if want := filepath.Join(tmp, "opencode.json"); got != want {
		t.Errorf("firstExistingOrDefault = %q, want %q", got, want)
	}
}

func mustTarget(t *testing.T, id string) mcpTarget {
	t.Helper()
	target, err := findMCPTarget(id)
	if err != nil {
		t.Fatalf("findMCPTarget(%q): %v", id, err)
	}
	return target
}
