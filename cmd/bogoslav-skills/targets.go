package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// family is one of the two MCP config shapes a canonical serverDescriptor
// is rendered into (TZ.md section 9.2).
type family int

const (
	// familyA is the mcpServers shape: claude, cursor, cline.
	familyA family = iota
	// familyB is the mcp shape (command as one array, "environment" not
	// "env"): opencode, kilo.
	familyB
)

// mcpTarget is everything install needs for one of the five live,
// MCP-capable targets: which config family it uses, the object key the
// server entry lives under, whether its config format has a documented
// per-server timeout field, and how to find its config file.
type mcpTarget struct {
	id        string
	family    family
	parentKey string
	// supportsTimeout is true only for a target whose own documentation
	// names a per-server timeout field in its config file (TZ.md section
	// 9.4): claude, opencode, kilo. It stays false for cline and cursor --
	// not because they are assumed to lack one, but because neither
	// tool's official docs name a field, a unit, or show one in a config
	// example; writing an unverified key would be a guess dressed up as a
	// fix. installOne uses this to decide whether entryJSON gets a
	// non-nil timeout for this target at all.
	supportsTimeout bool
	// resolveConfigPath returns the absolute path of this target's config
	// file given the project directory install is running against. Cline
	// ignores projectDir entirely: its config is a single file under the
	// user's home directory, not a project file (TZ.md section 9.2).
	resolveConfigPath func(projectDir string) (string, error)
}

// aiderTargetID is the one target with no MCP support and no Agent
// Skills support at all: it gets a CONVENTIONS.md instead (TZ.md section
// 9.3.5). It is a valid --target, just not an mcpTarget.
const aiderTargetID = "aider"

// mcpTargets holds the five live install targets that can be configured
// as an MCP server. Order matches TZ.md section 9.2's table and is what
// "install --all" iterates in.
var mcpTargets = []mcpTarget{
	{
		id:              "claude",
		family:          familyA,
		parentKey:       "mcpServers",
		supportsTimeout: true,
		resolveConfigPath: func(projectDir string) (string, error) {
			return filepath.Join(projectDir, ".mcp.json"), nil
		},
	},
	{
		id:              "opencode",
		family:          familyB,
		parentKey:       "mcp",
		supportsTimeout: true,
		resolveConfigPath: func(projectDir string) (string, error) {
			return firstExistingOrDefault(projectDir, "opencode.json", "opencode.jsonc"), nil
		},
	},
	{
		id:              "kilo",
		family:          familyB,
		parentKey:       "mcp",
		supportsTimeout: true,
		resolveConfigPath: func(projectDir string) (string, error) {
			return firstExistingOrDefault(projectDir, "kilo.jsonc", filepath.Join(".kilo", "kilo.jsonc")), nil
		},
	},
	{
		id:        "cline",
		family:    familyA,
		parentKey: "mcpServers",
		// supportsTimeout stays false: see the field's doc comment above.
		resolveConfigPath: func(string) (string, error) {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("resolve home directory for cline's config: %w", err)
			}
			return filepath.Join(home, ".cline", "mcp.json"), nil
		},
	},
	{
		id:        "cursor",
		family:    familyA,
		parentKey: "mcpServers",
		// supportsTimeout stays false: see the field's doc comment above.
		resolveConfigPath: func(projectDir string) (string, error) {
			return filepath.Join(projectDir, ".cursor", "mcp.json"), nil
		},
	},
}

// firstExistingOrDefault returns the first of candidates (each resolved
// under projectDir) that already exists on disk, or projectDir joined
// with the first candidate if none do. This lets opencode and kilo, both
// of which accept more than one config file name (TZ.md section 9.2),
// keep using whichever file the user already has instead of this command
// creating a second, competing one.
func firstExistingOrDefault(projectDir string, candidates ...string) string {
	for _, c := range candidates {
		p := filepath.Join(projectDir, c)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return filepath.Join(projectDir, candidates[0])
}

// ErrUnknownTarget is returned by findMCPTarget and by install's --target
// validation for any name that is not one of the six values validTargets
// lists.
var ErrUnknownTarget = errors.New("bogoslav-skills: unknown --target")

// validTargetIDs lists every accepted --target value: the five MCP-
// capable targets plus aider. Roo Code is deliberately absent: it was
// archived read-only by its owner on 2026-05-15 (TZ.md section 9.3.4)
// and is not a target this command will ever support.
func validTargetIDs() []string {
	ids := make([]string, 0, len(mcpTargets)+1)
	for _, t := range mcpTargets {
		ids = append(ids, t.id)
	}
	ids = append(ids, aiderTargetID)
	return ids
}

// findMCPTarget looks up id among mcpTargets. It returns ErrUnknownTarget
// for aiderTargetID too: aider has no MCP config to find, so callers that
// need an mcpTarget (rather than a plain --target string) must check for
// aider themselves before calling this.
func findMCPTarget(id string) (mcpTarget, error) {
	for _, t := range mcpTargets {
		if t.id == id {
			return t, nil
		}
	}
	return mcpTarget{}, fmt.Errorf("%w %q: must be one of %v", ErrUnknownTarget, id, validTargetIDs())
}

// validateTargetID returns an error unless id is exactly one of
// validTargetIDs -- in particular, it rejects "roo", "roo-code", and any
// other spelling of Roo Code by construction, since that name is simply
// never in the list (TZ.md section 9.3.4).
func validateTargetID(id string) error {
	for _, want := range validTargetIDs() {
		if id == want {
			return nil
		}
	}
	return fmt.Errorf("%w %q: must be one of %v", ErrUnknownTarget, id, validTargetIDs())
}

// sortedTargetIDs is validTargetIDs in alphabetical order, used only for
// stable, deterministic --help and error text.
func sortedTargetIDs() []string {
	ids := validTargetIDs()
	sort.Strings(ids)
	return ids
}
