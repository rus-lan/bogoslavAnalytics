package main

import (
	"bytes"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
)

// TestNewRootCmd_versionCommandPrintsAppVersionString proves
// bogoslav-mcp's `version` command prints the exact same text as
// bogoslav-cli's and bogoslav-skills' (TZ.md section 7.4), via
// internal/clitree.AddVersionSupport, and never starts the stdio server
// (run is never reachable from this path: it exits before ExecuteContext
// would call it). testLogger is server_test.go's own discard-*slog.Logger
// helper, reused here since newRootCmd needs one.
func TestNewRootCmd_versionCommandPrintsAppVersionString(t *testing.T) {
	root := newRootCmd(testLogger())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version: %v", err)
	}

	if want := app.VersionString(); out.String() != want {
		t.Errorf("version command output = %q, want %q", out.String(), want)
	}
}

// TestNewRootCmd_versionFlagMatchesVersionCommand proves --version
// prints identically to the version subcommand.
func TestNewRootCmd_versionFlagMatchesVersionCommand(t *testing.T) {
	root := newRootCmd(testLogger())
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version: %v", err)
	}

	if want := app.VersionString(); out.String() != want {
		t.Errorf("--version output = %q, want %q", out.String(), want)
	}
}
