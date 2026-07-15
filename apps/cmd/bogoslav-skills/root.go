package main

import "github.com/spf13/cobra"

// newRootCmd builds the bogoslav-skills command tree: generate (skill
// files only) and install (skill files plus, per target, an MCP
// registration or -- for aider -- CONVENTIONS.md).
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "bogoslav-skills",
		Short: "Generate the bogoslav Agent Skill and install it, plus the bogoslav-mcp registration, into an agent tool",
		Long: `bogoslav-skills does two jobs (TZ.md section 9):

  1. generate SKILL.md by walking apps/internal/clitree's live cobra
     command tree -- the same tree bogoslav-cli runs -- so command names,
     flags, and help text can never drift from the CLI.
  2. install that skill, and the bogoslav-mcp MCP server registration,
     into a target agent tool in one command.

Five live targets share one skill placement (.claude/skills/bogoslav/ and
.agents/skills/bogoslav/) and one of two MCP config shapes: claude,
cursor, and cline use "mcpServers"; opencode and kilo use "mcp" with
command as one array and "environment" instead of "env". A sixth target,
aider, has no MCP support at all and gets a degraded CONVENTIONS.md
instead. Every config file is merged, never overwritten: other MCP
servers, comments, and formatting already in it survive untouched.`,
		SilenceUsage: true,
	}

	root.AddCommand(
		newGenerateCmd(),
		newInstallCmd(),
	)

	return root
}
