package main

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/clitree"
)

// envNote is printed once per successful MCP config write, since
// newDescriptor never fills in Env/Environment (TZ.md section 2.5:
// GITLAB_URL/GITLAB_TOKEN are read from bogoslav-mcp's own environment,
// not from a config file) -- silently shipping an empty env block would
// leave a user wondering why their token never reached the server.
const envNote = "note: the merged entry's env/environment block is empty; bogoslav-mcp reads " +
	"GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it " +
	"needs to already have them set (or add them to that block yourself)"

// newInstallCmd builds the install command (TZ.md section 9): generate's
// skill-writing step, plus -- for every target but aider -- a merge of
// the bogoslav MCP server registration into that target's own config
// file (mcpconfig.go). Exactly one of --target or --all selects what to
// install for.
func newInstallCmd() *cobra.Command {
	var (
		target     string
		all        bool
		projectDir string
		mcpCommand string
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Write the SKILL.md and, except for aider, merge the bogoslav MCP server into a tool's config",
		Long: fmt.Sprintf(`install writes SKILL.md to both .claude/skills/bogoslav/ and
.agents/skills/bogoslav/ (the same step generate performs) and then, for
every target except aider, merges an MCP server registration for
bogoslav-mcp into that target's own config file -- never overwriting it,
only adding or updating the "%s" entry (TZ.md section 9.3.2).

aider has no MCP support at all: --target aider instead writes
CONVENTIONS.md (generated from the same command tree as SKILL.md) and
prints how to point aider at it.

Targets: %v.`, serverName, sortedTargetIDs()),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := selectedTarget(target, all)
			if err != nil {
				return err
			}

			resolvedCommand, err := resolveMCPCommand(mcpCommand)
			if err != nil {
				return fmt.Errorf("install: %w", err)
			}
			reportIfPathFallback(cmd.ErrOrStderr(), resolvedCommand)
			descriptor := newDescriptor(resolvedCommand)

			ids := []string{id}
			if all {
				ids = validTargetIDs()
			}

			for _, targetID := range ids {
				if err := installOne(cmd.OutOrStdout(), cmd.ErrOrStderr(), targetID, projectDir, descriptor, dryRun); err != nil {
					return fmt.Errorf("install %s: %w", targetID, err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&target, "target", "", fmt.Sprintf("install for one tool: %v", sortedTargetIDs()))
	cmd.Flags().BoolVar(&all, "all", false, "install for every target")
	cmd.Flags().StringVar(&projectDir, "project-dir", ".", "project directory to install into")
	cmd.Flags().StringVar(&mcpCommand, "mcp-command", "",
		"path to the bogoslav-mcp binary to register (default: auto-detected next to bogoslav-skills, or \"bogoslav-mcp\" on PATH)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing anything")

	return cmd
}

// selectedTarget resolves --target/--all into exactly one of: a single
// validated target id, or "" with all=true meaning every target. Exactly
// one of target/all must be set; validateTargetID is what rejects an
// unknown name (including any spelling of Roo Code, TZ.md section 9.3.4).
func selectedTarget(target string, all bool) (string, error) {
	switch {
	case target != "" && all:
		return "", fmt.Errorf("bogoslav-skills: --target and --all are mutually exclusive")
	case target == "" && !all:
		return "", fmt.Errorf("bogoslav-skills: pass --target <tool> or --all; targets are %v", sortedTargetIDs())
	case all:
		return "", nil
	default:
		if err := validateTargetID(target); err != nil {
			return "", err
		}
		return target, nil
	}
}

// installOne installs targetID: aider gets CONVENTIONS.md and no MCP
// registration (TZ.md section 9.3.5); every other target gets the skill
// written to both directories (writer.go) and its own config file merged
// (mcpconfig.go). Every change -- create, update, or unchanged -- is
// reported on out as it happens, so a long --all run is not silent.
func installOne(out, errOut io.Writer, targetID, projectDir string, descriptor serverDescriptor, dryRun bool) error {
	if targetID == aiderTargetID {
		return installAider(out, errOut, projectDir, dryRun)
	}

	t, err := findMCPTarget(targetID)
	if err != nil {
		return err
	}

	skillChanges, err := writeSkillFiles(projectDir, renderSkillMarkdown(clitree.NewRootCmd()), dryRun)
	if err != nil {
		return err
	}
	for _, c := range skillChanges {
		fmt.Fprintln(out, c.String())
	}

	configPath, err := t.resolveConfigPath(projectDir)
	if err != nil {
		return err
	}
	entry, err := entryJSON(t.family, descriptor)
	if err != nil {
		return err
	}
	configChange, err := mergeConfigFile(configPath, t.parentKey, descriptor.Name, entry, dryRun)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, configChange.String())
	if configChange.kind != changeUnchanged {
		fmt.Fprintln(errOut, envNote)
	}
	return nil
}

// installAider writes CONVENTIONS.md at projectDir's root and reports how
// to point aider at it; it never touches SKILL.md or any MCP config
// (TZ.md section 9.3.5: aider does not read Agent Skills and has no MCP
// support at all).
func installAider(out, errOut io.Writer, projectDir string, dryRun bool) error {
	content := renderConventionsMarkdown(clitree.NewRootCmd())
	c, err := writeFileTracked(filepath.Join(projectDir, "CONVENTIONS.md"), []byte(content), dryRun)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, c.String())
	fmt.Fprint(errOut, conventionsUsageNote)
	return nil
}
