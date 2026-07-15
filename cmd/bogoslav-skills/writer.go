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
	// real is set only when path is currently a symlink: it is the
	// absolute path the read and write below actually followed it to.
	// writeFileTracked always follows a symlinked config rather than
	// refusing it outright -- that is a legitimate, common setup
	// (dotfile managers such as GNU Stow or chezmoi symlink config files
	// into place all the time, and cline's own config already lives
	// outside the project tree by design) -- but it never does so
	// silently: real is what lets every caller print where a write
	// actually landed, so a config that turns out to be a symlink
	// pointing outside the tree the user ran install against is always
	// visible in the command's own output.
	real string
}

func (c change) String() string {
	label := c.path
	if c.real != "" {
		label = fmt.Sprintf("%s (symlink -> %s)", c.path, c.real)
	}
	switch c.kind {
	case changeCreated:
		return fmt.Sprintf("create %s", label)
	case changeUpdated:
		return fmt.Sprintf("update %s", label)
	default:
		return fmt.Sprintf("unchanged %s", label)
	}
}

// symlinkTarget reports whether path is currently a symlink and, if so,
// the absolute, fully-resolved path it points to. A path that does not
// exist yet, or that is a plain file or directory, is reported as
// ("", nil): not a symlink.
//
// A symlink whose target does not exist (or whose chain hits a missing
// target partway through) cannot be resolved by filepath.EvalSymlinks;
// symlinkTarget falls back to the link's own immediate target (still
// made absolute) so a caller always has something concrete to show the
// user, even for a dangling symlink.
func symlinkTarget(path string) (real string, err error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("stat %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path for %q: %w", path, err)
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		return resolved, nil
	}

	link, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("read link %q: %w", path, err)
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(absPath), link)
	}
	return filepath.Clean(link), nil
}

// writeFileTracked writes content to path, creating any missing parent
// directories, unless dryRun is set -- in which case it only reports what
// it would have done. It never writes when content already matches what
// is on disk, which is what makes installing twice idempotent (TZ.md
// section 9.3, "installing twice must be idempotent"): the second run's
// merged bytes are identical to the first run's, so this reports
// changeUnchanged and touches nothing.
//
// If path is a symlink, os.ReadFile and os.WriteFile below follow it --
// the same as any other tool that opens path by name -- so the bytes
// actually land at the symlink's target, not at path itself.
// writeFileTracked resolves that target up front (symlinkTarget) purely
// to report it: see change.real.
func writeFileTracked(path string, content []byte, dryRun bool) (change, error) {
	real, err := symlinkTarget(path)
	if err != nil {
		return change{}, err
	}

	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if bytes.Equal(existing, content) {
			return change{path: path, kind: changeUnchanged, real: real}, nil
		}
		if dryRun {
			return change{path: path, kind: changeUpdated, real: real}, nil
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return change{}, fmt.Errorf("write %q: %w", path, err)
		}
		return change{path: path, kind: changeUpdated, real: real}, nil
	case errors.Is(err, os.ErrNotExist):
		if dryRun {
			return change{path: path, kind: changeCreated, real: real}, nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return change{}, fmt.Errorf("create directory for %q: %w", path, err)
		}
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return change{}, fmt.Errorf("write %q: %w", path, err)
		}
		return change{path: path, kind: changeCreated, real: real}, nil
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
