package clitree

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/app"
)

// NewVersionCmd builds the `version` command: prints app.VersionString()
// (this build's resolved version plus the Go toolchain it was built
// with, or an explanation instead of a bare nonce for an unreleased,
// dirty-tree build -- see internal/app/version.go) and exits.
//
// It is deliberately NOT one of NewRootCmd's six pipeline commands, and
// NewRootCmd does not add it: `version` mirrors no internal/app use case
// and no MCP tool, but NewRootCmd's own tree is also exactly what
// bogoslav-skills walks to generate SKILL.md/CONVENTIONS.md (renderSkillMarkdown,
// renderConventionsMarkdown) and one entry per MCP tool one-to-one --
// adding `version` there would make both documents claim an MCP tool
// that bogoslav-mcp never registers (TZ.md section 7.4). Callers attach
// it separately, via AddVersionSupport, to whichever root they actually
// execute.
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print this build's version",
		Long: `version prints this build's own resolved version, plus the Go
toolchain it was built with, to help tell a released binary from a local
build when reporting a bug. bogoslav-cli, bogoslav-mcp and bogoslav-skills
all print the exact same text: whichever of the three you run this on
answers with the same version.`,
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), app.VersionString())
			return err
		},
	}
}

// AddVersionSupport attaches both version surfaces this task calls for
// to root: the `version` subcommand (NewVersionCmd) and, via cobra's own
// built-in mechanism (Command.Version non-empty auto-registers a
// --version/-v flag), a --version flag. Both print byte-for-byte the
// same text -- SetVersionTemplate is given app.VersionString() itself,
// not a template referencing root.Version, so there is no second,
// hand-copied rendering path for either surface to drift from.
//
// Call this on the root each binary's own main actually executes, never
// on the tree NewRootCmd returns for bogoslav-cli: bogoslav-skills also
// walks that exact tree (a fresh instance, built again by its own call
// to NewRootCmd) to generate SKILL.md/CONVENTIONS.md, and this function
// is never called on that copy, so generation is unaffected either way.
func AddVersionSupport(root *cobra.Command) {
	root.AddCommand(NewVersionCmd())
	root.Version = app.ToolVersion
	root.SetVersionTemplate(app.VersionString())
}
