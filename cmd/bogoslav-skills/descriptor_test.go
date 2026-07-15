package main

import (
	"path/filepath"
	"testing"
)

func TestNewDescriptor_argsAndEnvAreNeverNil(t *testing.T) {
	d := newDescriptor("/path/to/bogoslav-mcp")

	if d.Args == nil {
		t.Error("Args is nil, want an empty (non-nil) slice so JSON marshals \"[]\", not \"null\"")
	}
	if d.Env == nil {
		t.Error("Env is nil, want an empty (non-nil) map so JSON marshals \"{}\", not \"null\"")
	}
	if d.Name != serverName {
		t.Errorf("Name = %q, want %q", d.Name, serverName)
	}
	if d.Command != "/path/to/bogoslav-mcp" {
		t.Errorf("Command = %q, want %q", d.Command, "/path/to/bogoslav-mcp")
	}
}

func TestSiblingOrPathFallback_findsSiblingBinary(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "bogoslav-skills")
	sibling := filepath.Join(dir, mcpBinaryName())
	writeTestFile(t, sibling, "#!/bin/sh\n")

	got := siblingOrPathFallback(self)
	if got != sibling {
		t.Errorf("siblingOrPathFallback = %q, want %q", got, sibling)
	}
}

func TestSiblingOrPathFallback_fallsBackToBareNameWhenNoSiblingExists(t *testing.T) {
	dir := t.TempDir()
	self := filepath.Join(dir, "bogoslav-skills")

	got := siblingOrPathFallback(self)
	if got != mcpBinaryName() {
		t.Errorf("siblingOrPathFallback = %q, want the bare PATH fallback %q", got, mcpBinaryName())
	}
}

func TestResolveMCPCommand_explicitAlwaysWins(t *testing.T) {
	got, err := resolveMCPCommand("/explicit/path")
	if err != nil {
		t.Fatalf("resolveMCPCommand: %v", err)
	}
	if got != "/explicit/path" {
		t.Errorf("resolveMCPCommand(explicit) = %q, want %q", got, "/explicit/path")
	}
}
