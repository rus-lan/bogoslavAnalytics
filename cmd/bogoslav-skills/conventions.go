package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// renderConventionsMarkdown generates aider's degraded target (TZ.md
// section 9.3.5): aider has no MCP support at all (verified: zero MCP
// options in its config reference, PR #3672 closed unmerged), so instead
// of a skill and an MCP registration it gets a plain conventions file
// documenting bogoslav-cli, generated from the exact same root
// (internal/clitree.NewRootCmd() in production) as SKILL.md -- never
// a hand-copied second description of the same commands.
func renderConventionsMarkdown(root *cobra.Command) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n%s\n\n", root.Short, root.Long)
	b.WriteString("bogoslav-cli has no MCP integration for aider to use (aider does not " +
		"support MCP at all). Run these commands yourself, or have aider run them, and feed " +
		"aider the resulting artifact files; do not expect aider to call bogoslav-cli on its " +
		"own initiative beyond what you ask for in a message.\n\n")

	b.WriteString("## Commands\n\n")
	for _, cmd := range root.Commands() {
		fmt.Fprintf(&b, "### `%s`\n\n", cmd.Name())
		if cmd.Short != "" {
			fmt.Fprintf(&b, "%s\n\n", cmd.Short)
		}
		if cmd.Long != "" {
			fmt.Fprintf(&b, "%s\n\n", cmd.Long)
		}
		renderFlags(&b, cmd)
	}

	return b.String()
}

// conventionsUsageNote is printed to stderr alongside CONVENTIONS.md's
// change report, telling the user how to actually get aider to read it
// (TZ.md section 9.3.5): aider never reads a file on its own, it has to
// be told to.
const conventionsUsageNote = `aider does not read CONVENTIONS.md on its own -- point aider at it with
either:
  aider --read CONVENTIONS.md
or add to .aider.conf.yml:
  read: [CONVENTIONS.md]
`
