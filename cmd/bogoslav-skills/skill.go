package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// subagentGuidance is the one piece of SKILL.md content the cobra tree
// has no way to express (TZ.md sections 8.2, 9.1): which agent or model
// should do the semantic labeling step, and that bogoslav-mcp/bogoslav-cli
// never call a model themselves. Everything else in renderSkillMarkdown
// is generated from internal/clitree.
const subagentGuidance = `## Labeling: who does it and how

bogoslav never labels a comment itself: get-classify-batch (MCP tool
get_classify_batch) only hands back the batch of comments, the taxonomy,
the labeling result's JSON Schema, and a rendered prompt. The calling
agent labels every comment against that schema -- inline with the current
model, or by spawning a specialized sub-agent for classification if the
environment has one -- and then passes the result to save-labels (MCP
tool save_labels), which validates it against the same schema and only on
success writes the labeled_comments artifact. A labeling that fails
validation (a label outside the taxonomy, or an extra, missing, or
duplicate note_id) writes no file and returns every violation at once, so
fix all of them before retrying rather than resubmitting one at a time.
`

// mcpToolName derives an MCP tool's snake_case name from its mirroring
// CLI command's kebab-case name (TZ.md section 7.3: tool names are
// snake_case, deliberately distinct from bogoslav-cli's kebab-case
// commands). This is a pure, mechanical transform, not a hand-copied
// table, and it produces exactly bogoslav-mcp's six registered tool
// names: find_mrs, get_comments, get_classify_batch, save_labels,
// filter_comments, get_stats.
func mcpToolName(cliCommandName string) string {
	return strings.ReplaceAll(cliCommandName, "-", "_")
}

// renderSkillMarkdown generates SKILL.md's full content by walking root
// (internal/clitree.NewRootCmd() in production): frontmatter and the
// top-level overview come straight from root.Short/root.Long, and one
// section per subcommand comes from that command's own Short, Long, and
// flags (TZ.md section 9.1: "содержимое SKILL.md генерируется из дерева
// команд cobra"). Regenerating from an unchanged tree reproduces this
// output exactly; renaming a command, changing a flag, or editing help
// text changes it (TZ.md section 12, criterion 14) because none of that
// text is duplicated here by hand.
//
// It deliberately takes no serverDescriptor: the resolved bogoslav-mcp
// executable path is an install-time, per-machine config detail (it goes
// into the target tool's own config file, see mcpconfig.go), not skill
// content, so SKILL.md never embeds it and stays identical across
// machines for the same cobra tree.
func renderSkillMarkdown(root *cobra.Command) string {
	var b strings.Builder

	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n", serverName, root.Short)
	fmt.Fprintf(&b, "# %s\n\n%s\n\n", root.Short, root.Long)

	b.WriteString(subagentGuidance)
	b.WriteString("\n## MCP server\n\n")
	fmt.Fprintf(&b, "Install this skill's MCP server with `bogoslav-skills install --target <tool>` "+
		"(or `--all`); it is then registered under the name `%s` and runs over stdio. Every "+
		"command below is available both as an MCP tool (snake_case name, in parentheses) "+
		"and as the equivalent `bogoslav-cli` command (kebab-case, the heading itself) -- "+
		"the two are thin wrappers over the exact same function, so they never disagree.\n\n", serverName)

	b.WriteString("## Commands\n\n")
	for _, cmd := range root.Commands() {
		fmt.Fprintf(&b, "### `%s` (MCP tool `%s`)\n\n", cmd.Name(), mcpToolName(cmd.Name()))
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

// renderFlags lists every flag cmd.Flags() carries, in the same
// lexicographic order pflag.FlagSet.VisitAll always visits them in, so
// regenerating twice from an unchanged tree never reorders this section.
func renderFlags(b *strings.Builder, cmd *cobra.Command) {
	var flags []*pflag.Flag
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		flags = append(flags, f)
	})
	if len(flags) == 0 {
		return
	}

	b.WriteString("Flags:\n\n")
	for _, f := range flags {
		fmt.Fprintf(b, "- `--%s` (%s%s): %s\n", f.Name, f.Value.Type(), requiredSuffix(f), f.Usage)
	}
	b.WriteString("\n")
}

// requiredSuffix reports a flag as required exactly when
// cmd.MarkFlagRequired put it there: cobra records that as the
// f.Annotations[cobra.BashCompOneRequiredFlag] annotation, the same one
// its own shell-completion code reads.
func requiredSuffix(f *pflag.Flag) string {
	if _, ok := f.Annotations[cobra.BashCompOneRequiredFlag]; ok {
		return ", required"
	}
	return ""
}
