package main

import (
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslavAnalytics/internal/clitree"
)

// envNote is printed once per successful MCP config write, since
// newDescriptor never fills in Env/Environment (TZ.md section 2.5:
// GITLAB_URL/GITLAB_TOKEN are read from bogoslav-mcp's own environment,
// not from a config file) -- silently shipping an empty env block would
// leave a user wondering why their token never reached the server.
const envNote = "note: the merged entry's env/environment block is empty; bogoslav-mcp reads " +
	"GITLAB_URL and GITLAB_TOKEN from its own process environment, so whatever spawns it " +
	"needs to already have them set (or add them to that block yourself)"

// defaultMCPTimeout is the per-server timeout install writes, in
// milliseconds, into a target's config for a target whose format
// documents such a field (TZ.md section 9.4: claude, opencode, kilo).
// Chosen from a real slow-GitLab bruteforce run, not a round guess: a
// user's report was a bruteforce find-mrs walking 28 pages of merge
// requests plus one /discussions call per surviving candidate, which
// legitimately runs well past the shortest defaults these tools ship
// (opencode/kilo document a 5-30 second timeout for their own field;
// Claude Code's stdio idle-abort defaults to 30 minutes). An hour
// comfortably covers that shape of run with headroom to spare, while
// --mcp-timeout still lets a user on an even slower instance raise it
// further.
const defaultMCPTimeout = time.Hour

// validateMCPTimeout rejects a non-positive --mcp-timeout: zero or
// negative would tell a supporting target's client to give up
// immediately (or would mean nothing at all), defeating the point of
// the flag.
func validateMCPTimeout(d time.Duration) error {
	if d <= 0 {
		return fmt.Errorf("--mcp-timeout must be positive, got %s", d)
	}
	return nil
}

// timeoutNote is printed after a successful config write for a target
// that has a documented per-server timeout field (TZ.md section 9.4):
// it names the value written, in both the human duration the user
// passed and the milliseconds actually on disk, and how to change it.
func timeoutNote(d time.Duration) string {
	return fmt.Sprintf("note: wrote a %s (%dms) per-call MCP timeout into the merged entry; "+
		"raise it with --mcp-timeout for an even slower GitLab instance", d, d.Milliseconds())
}

// noTimeoutNote is printed instead of timeoutNote for a target whose
// config format has no documented per-server timeout field (TZ.md
// section 9.4: cline, cursor). bogoslav-skills cannot raise a
// client-imposed deadline it has no config key for, and says so instead
// of silently writing nothing and leaving the user to assume it helped.
func noTimeoutNote(targetID string) string {
	return fmt.Sprintf("note: %s has no documented MCP per-call timeout field, so bogoslav-skills did not write one; "+
		"its own client-side deadline cannot be raised from here -- narrow the query with --group/--project "+
		"instead of an instance-wide search, or check whether %s's own settings expose a longer timeout",
		targetID, targetID)
}

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
		mcpTimeout time.Duration
		dryRun     bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Write the SKILL.md and, except for aider, merge the bogoslav MCP server into a tool's config",
		Long: fmt.Sprintf(`install writes SKILL.md to both .claude/skills/bogoslav/ and
.agents/skills/bogoslav/ (the same step generate performs) and then, for
every target except aider, merges an MCP server registration for
bogoslav-mcp into that target's own config file -- never overwriting it,
only adding or updating the "%s" entry.

Three of those five targets -- claude, opencode, kilo -- document a
per-server timeout field in their own config format; install writes a
%s timeout into it by default, raised with --mcp-timeout for an even
slower GitLab instance. cline and cursor document no such field at all:
install writes nothing for it there, since a client-side tool-call
deadline this command cannot see cannot be raised from a config key
that does not exist -- narrow the query with --group/--project
instead, or check whether that tool's own settings expose a timeout.

aider has no MCP support at all: --target aider instead writes
CONVENTIONS.md (generated from the same command tree as SKILL.md) and
prints how to point aider at it.

Targets: %v.`, serverName, defaultMCPTimeout, sortedTargetIDs()),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := selectedTarget(target, all)
			if err != nil {
				return err
			}
			if err := validateMCPTimeout(mcpTimeout); err != nil {
				return fmt.Errorf("install: %w", err)
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
				if err := installOne(cmd.OutOrStdout(), cmd.ErrOrStderr(), targetID, projectDir, descriptor, mcpTimeout, dryRun); err != nil {
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
	cmd.Flags().DurationVar(&mcpTimeout, "mcp-timeout", defaultMCPTimeout,
		"per-call MCP timeout to write for a target whose config format has one (claude, opencode, kilo); "+
			"raise this for an even slower GitLab instance, for example \"2h\" (has no effect on cline or cursor, "+
			"which document no such field)")
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
// (mcpconfig.go). mcpTimeout is only ever written for a target whose
// config format documents a timeout field (t.supportsTimeout, TZ.md
// section 9.4) -- installOne resolves that per call, entryJSON never
// guesses. Every change -- create, update, or unchanged -- is reported
// on out as it happens, so a long --all run is not silent.
func installOne(out, errOut io.Writer, targetID, projectDir string, descriptor serverDescriptor, mcpTimeout time.Duration, dryRun bool) error {
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
	var timeoutMillis *int64
	if t.supportsTimeout {
		millis := mcpTimeout.Milliseconds()
		timeoutMillis = &millis
	}
	entry, err := entryJSON(t.family, descriptor, timeoutMillis)
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
		if t.supportsTimeout {
			fmt.Fprintln(errOut, timeoutNote(mcpTimeout))
		} else {
			fmt.Fprintln(errOut, noTimeoutNote(t.id))
		}
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
