package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/clitree"
)

// newGenerateCmd builds the generate-only command (TZ.md section 9,
// "also useful: a generate-only mode that writes the skill files without
// touching any tool config"): it writes SKILL.md to both
// .claude/skills/bogoslav/ and .agents/skills/bogoslav/ and stops there.
// install (install.go) does this same step and then goes on to merge the
// target's MCP config.
func newGenerateCmd() *cobra.Command {
	var projectDir string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Write SKILL.md, generated from bogoslav-cli's command tree, without installing anything",
		Long: `generate renders SKILL.md from apps/internal/clitree's live command
tree -- the same tree bogoslav-cli itself runs -- and writes it to both
.claude/skills/bogoslav/SKILL.md and .agents/skills/bogoslav/SKILL.md
under --project-dir. It never touches an MCP config file: use install for
that.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			content := renderSkillMarkdown(clitree.NewRootCmd())

			changes, err := writeSkillFiles(projectDir, content, false)
			if err != nil {
				return fmt.Errorf("generate: %w", err)
			}
			for _, c := range changes {
				fmt.Fprintln(cmd.OutOrStdout(), c.String())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory to write the skill under")

	return cmd
}
