package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// changeKind is what writeFileTracked did (or, on --dry-run, would do) to
// one file.
type changeKind int

const (
	changeUnchanged changeKind = iota
	changeCreated
	changeUpdated
)

// change reports one file's outcome, for both --dry-run output and the
// normal run's summary.
type change struct {
	path string
	kind changeKind
}

func (c change) String() string {
	switch c.kind {
	case changeCreated:
		return fmt.Sprintf("create %s", c.path)
	case changeUpdated:
		return fmt.Sprintf("update %s", c.path)
	default:
		return fmt.Sprintf("unchanged %s", c.path)
	}
}

// writeFileTracked writes content to path, creating any missing parent
// directories, unless dryRun is set -- in which case it only reports what
// it would have done. It never writes when content already matches what
// is on disk, which is what makes installing twice idempotent (TZ.md
// section 9.3, "installing twice must be idempotent"): the second run's
// merged bytes are identical to the first run's, so this reports
// changeUnchanged and touches nothing.
func writeFileTracked(path string, content []byte, dryRun bool) (change, error) {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if bytes.Equal(existing, content) {
			return change{path, changeUnchanged}, nil
		}
		if dryRun {
			return change{path, changeUpdated}, nil
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return change{}, fmt.Errorf("write %q: %w", path, err)
		}
		return change{path, changeUpdated}, nil
	case errors.Is(err, os.ErrNotExist):
		if dryRun {
			return change{path, changeCreated}, nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return change{}, fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return change{}, fmt.Errorf("write %q: %w", path, err)
		}
		return change{path, changeCreated}, nil
	default:
		return change{}, fmt.Errorf("read %q: %w", path, err)
	}
}

// writeSkillFiles writes content -- SKILL.md's rendered markdown -- to
// both directories every one of the five live targets can read it from
// (TZ.md section 9.1): .claude/skills/bogoslav/ and
// .agents/skills/bogoslav/, under projectDir. Both copies are always
// written together and are always byte-identical to each other.
func writeSkillFiles(projectDir, content string, dryRun bool) ([]change, error) {
	paths := []string{
		filepath.Join(projectDir, ".claude", "skills", serverName, "SKILL.md"),
		filepath.Join(projectDir, ".agents", "skills", serverName, "SKILL.md"),
	}
	changes := make([]change, 0, len(paths))
	for _, p := range paths {
		c, err := writeFileTracked(p, []byte(content), dryRun)
		if err != nil {
			return changes, err
		}
		changes = append(changes, c)
	}
	return changes, nil
}

// mergeConfigFile reads path (treating a missing file as empty), upserts
// the descriptor's name/parentKey entry into it via mergeStdioServer, and
// writes the result back through writeFileTracked -- the one path a
// target's MCP config file is ever read, merged, or written on.
func mergeConfigFile(path, parentKey, name string, entry []byte, dryRun bool) (change, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return change{}, fmt.Errorf("read %q: %w", path, err)
	}

	merged, err := mergeStdioServer(existing, parentKey, name, entry)
	if err != nil {
		return change{}, fmt.Errorf("merge %q: %w", path, err)
	}

	return writeFileTracked(path, merged, dryRun)
}
