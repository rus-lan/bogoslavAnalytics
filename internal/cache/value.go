package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ExtValue is the file extension Put uses for the small keyed scalar
// entries in this file: values with their own checked_at, not one of
// the four artifact kinds (TZ.md sections 4, 4.6). No artifact format
// ever produces this extension — ExtYAML, ExtJSON, ExtText, and the
// two html spellings (".html", ".htm") are the only ones a written
// artifact ever carries (TZ.md section 4) — so valueFileName can never
// collide with an artifact file name, for any name/hash pair on the
// value side and any kind/hash pair on the artifact side, regardless
// of what name the caller picks (even a name equal to a real artifact
// kind, like "mr_list", is safe: the extension still differs).
const ExtValue = "value"

// valueFileName builds the file name for a single stored value: the
// same "<name>_<hash>.<ext>" shape FileName builds for artifacts
// (TZ.md section 4.5), scoped by name instead of kind, always with
// ExtValue.
func valueFileName(name, hash string) string {
	return FileName(name, hash, ExtValue)
}

// valueEntry is the on-disk envelope Put writes and Get reads for a
// single stored value: the value itself, plus the moment it was
// checked. checked_at is what freshness (TTL) is measured against —
// not the file's own mtime, the same choice artifact.Source.FetchedAt
// makes for artifacts (TZ.md section 4.1).
type valueEntry[T any] struct {
	CheckedAt time.Time `json:"checked_at"`
	Value     T         `json:"value"`
}

// Get reads back the value most recently stored under hash by Put,
// scoped to name — the same role a "kind" plays for artifact Lookup:
// pick a distinct name per call site (e.g. "resolved_user",
// "smoke_test") so two unrelated value caches sharing opts.Dir never
// read each other's entries, even in the astronomically unlikely event
// of a SHA-256 collision between their hashes.
//
// A hit requires all of: opts.Refresh unset, the entry file present,
// readable, decodable as the expected JSON shape, and younger than
// opts.TTL (checked the same way Lookup does: now.Sub(checkedAt) <
// opts.TTL). Every other outcome — Refresh set, the file missing, a
// permission error or any other read failure, or bytes that fail to
// decode as valueEntry[T] — is reported identically: hit=false, value
// is T's zero value, no error returned.
//
// This is deliberate and load-bearing: a value entry is a performance
// optimization over redoing the real check, never a source of truth,
// so a corrupt or unreadable entry must never surface as an error that
// stops bogoslav-cli/bogoslav-mcp from doing the work it was about to
// do anyway. The worst outcome of a broken entry is one extra GitLab
// round trip, never a crash.
func Get[T any](name, hash string, opts Options, now time.Time) (value T, hit bool) {
	var zero T
	if opts.Refresh {
		return zero, false
	}

	path := filepath.Join(opts.Dir, valueFileName(name, hash))
	raw, err := os.ReadFile(path)
	if err != nil {
		return zero, false
	}

	var e valueEntry[T]
	if err := json.Unmarshal(raw, &e); err != nil {
		return zero, false
	}

	if now.Sub(e.CheckedAt) >= opts.TTL {
		return zero, false
	}
	return e.Value, true
}

// Put stores value under hash, scoped to name, recording now as its
// checked_at moment (TZ.md sections 4.6, 5.0, 5.5).
//
// Put writes to a temporary file inside opts.Dir and renames it into
// the target path, rather than writing the target path directly:
// os.Rename replaces its destination atomically on the same filesystem
// (POSIX rename(2)), so a concurrent Get against the same hash only
// ever observes the old complete entry or the new complete entry,
// never a partially written one.
//
// This matters for exactly one of the two processes this store exists
// for. bogoslav-cli is one process per invocation — the whole reason
// this package persists to disk instead of caching in memory — so
// within a single run there is at most one writer per hash and nothing
// to race with. bogoslav-mcp is different: it is one long-running
// process for its whole session, and the MCP SDK does not promise to
// serve tool calls one at a time, so two overlapping find_mrs/
// get_comments calls for the same user could plausibly race to Put the
// same hash. The atomic rename means that race can only ever produce
// "last write wins" — never a torn or corrupt entry, the same
// guarantee every Get gets regardless of how many concurrent writers
// there are. Callers do not need their own locking for this store to
// stay correct; a lock might still save the losing write's work, but
// that is a performance concern, not a correctness one, and is left to
// the caller.
func Put[T any](name, hash string, value T, opts Options, now time.Time) error {
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return fmt.Errorf("cache put %s: make dir %s: %w", name, opts.Dir, err)
	}

	raw, err := json.Marshal(valueEntry[T]{CheckedAt: now, Value: value})
	if err != nil {
		return fmt.Errorf("cache put %s: marshal value: %w", name, err)
	}

	tmp, err := os.CreateTemp(opts.Dir, ".tmp-"+name+"-*")
	if err != nil {
		return fmt.Errorf("cache put %s: create temp file: %w", name, err)
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup: a no-op once the rename below has succeeded
	// (tmpPath no longer exists under its own name by then), and the
	// only thing that matters if we return early with an error above.
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("cache put %s: write temp file: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("cache put %s: close temp file: %w", name, err)
	}

	path := filepath.Join(opts.Dir, valueFileName(name, hash))
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("cache put %s: rename into place: %w", name, err)
	}
	return nil
}
