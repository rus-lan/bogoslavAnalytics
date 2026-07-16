package clitree

import (
	"bytes"
	"testing"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
)

// TestNewRootCmd_hasNoVersionCommand pins the design decision documented
// on NewVersionCmd: NewRootCmd's own tree stays exactly the six pipeline
// commands (TestNewRootCmd_hasExactlySixExpectedCommands, root_test.go)
// with no `version` command mixed in, because bogoslav-skills walks this
// exact tree to generate SKILL.md/CONVENTIONS.md, one section per
// (command, matching MCP tool) pair -- a `version` entry there would
// claim an MCP tool bogoslav-mcp never registers.
func TestNewRootCmd_hasNoVersionCommand(t *testing.T) {
	for _, cmd := range NewRootCmd().Commands() {
		if cmd.Name() == "version" {
			t.Fatal("NewRootCmd().Commands() contains \"version\"; it must be attached separately via AddVersionSupport, never inside NewRootCmd itself")
		}
	}
}

// TestNewVersionCmd_printsAppVersionString proves the version command
// writes exactly app.VersionString(), through cmd.OutOrStdout() -- never
// os.Stdout or fmt.Print* directly, which cmd/bogoslav-mcp's
// TestPackage_neverWritesToStdoutDirectly would otherwise forbid if this
// package were ever pulled into that binary's dependency graph in a way
// that used them.
func TestNewVersionCmd_printsAppVersionString(t *testing.T) {
	cmd := NewVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(nil)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := app.VersionString()
	if out.String() != want {
		t.Errorf("version command output = %q, want %q", out.String(), want)
	}
}

// TestNewVersionCmd_rejectsArgs proves `version` takes no positional
// arguments, so a typo like `version now` fails loudly instead of being
// silently ignored.
func TestNewVersionCmd_rejectsArgs(t *testing.T) {
	cmd := NewVersionCmd()
	cmd.SetOut(new(bytes.Buffer))
	cmd.SetErr(new(bytes.Buffer))
	cmd.SetArgs([]string{"extra-arg"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute() with an extra positional arg succeeded, want an error")
	}
}

// TestAddVersionSupport_versionFlagMatchesVersionCommand proves both
// surfaces AddVersionSupport attaches -- the `version` subcommand and
// the --version flag -- print identically: neither is a second,
// hand-copied rendering of the other.
func TestAddVersionSupport_versionFlagMatchesVersionCommand(t *testing.T) {
	subcommandRoot := NewRootCmd()
	AddVersionSupport(subcommandRoot)
	var subcommandOut bytes.Buffer
	subcommandRoot.SetOut(&subcommandOut)
	subcommandRoot.SetArgs([]string{"version"})
	if err := subcommandRoot.Execute(); err != nil {
		t.Fatalf("Execute([]string{\"version\"}) error = %v", err)
	}

	flagRoot := NewRootCmd()
	AddVersionSupport(flagRoot)
	var flagOut bytes.Buffer
	flagRoot.SetOut(&flagOut)
	flagRoot.SetArgs([]string{"--version"})
	if err := flagRoot.Execute(); err != nil {
		t.Fatalf("Execute([]string{\"--version\"}) error = %v", err)
	}

	if subcommandOut.String() != flagOut.String() {
		t.Errorf("`version` printed %q, --version printed %q, want them equal", subcommandOut.String(), flagOut.String())
	}
	if subcommandOut.String() != app.VersionString() {
		t.Errorf("`version` printed %q, want app.VersionString() %q", subcommandOut.String(), app.VersionString())
	}
}

// TestAddVersionSupport_doesNotChangeNewRootCmdsOwnSixCommands proves
// AddVersionSupport, called on one instance NewRootCmd returns, cannot
// affect a second, independent instance -- exactly the property
// generate.go/install.go rely on: each of their own calls to
// clitree.NewRootCmd() builds a fresh *cobra.Command (root.go), so
// cmd/bogoslav-cli attaching version support to ITS copy, in its own
// main, never leaks into SKILL.md/CONVENTIONS.md generation.
func TestAddVersionSupport_doesNotChangeNewRootCmdsOwnSixCommands(t *testing.T) {
	augmented := NewRootCmd()
	AddVersionSupport(augmented)

	fresh := NewRootCmd()
	if len(fresh.Commands()) != 6 {
		t.Fatalf("a fresh NewRootCmd() after a different instance got AddVersionSupport = %d commands, want 6", len(fresh.Commands()))
	}
	for _, cmd := range fresh.Commands() {
		if cmd.Name() == "version" {
			t.Fatal("a fresh NewRootCmd() picked up \"version\" from a different instance; AddVersionSupport must not have any shared/global effect")
		}
	}
}
