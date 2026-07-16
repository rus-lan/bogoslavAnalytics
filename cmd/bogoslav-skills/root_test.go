package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
)

func TestRootCmd_helpListsBothSubcommands(t *testing.T) {
	root := newRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help: %v", err)
	}

	for _, want := range []string{"generate", "install"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("--help output is missing %q:\n%s", want, out.String())
		}
	}
}

func TestRootCmd_unknownCommandFails(t *testing.T) {
	root := newRootCmd()
	root.SetOut(new(bytes.Buffer))
	root.SetErr(new(bytes.Buffer))
	root.SetArgs([]string{"not-a-real-command"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected an error for an unknown subcommand")
	}
}

// TestRootCmd_versionCommandPrintsAppVersionString proves bogoslav-skills'
// `version` command prints the exact same text as bogoslav-cli's and
// bogoslav-mcp's, via internal/clitree.AddVersionSupport (TZ.md section 7.4).
func TestRootCmd_versionCommandPrintsAppVersionString(t *testing.T) {
	root := newRootCmd()
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

// TestRootCmd_versionFlagMatchesVersionCommand proves --version prints
// identically to the version subcommand.
func TestRootCmd_versionFlagMatchesVersionCommand(t *testing.T) {
	root := newRootCmd()
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
