package cache

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPut_thenGetReturnsValue(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	if err := Put("resolved_user", "abc123", int64(42), Options{Dir: dir, TTL: DefaultTTL}, now); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL}, now.Add(time.Minute))
	if !hit {
		t.Fatal("Get() hit = false, want true")
	}
	if got != 42 {
		t.Errorf("Get() value = %d, want 42", got)
	}
}

func TestGet_expiredEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	writtenAt := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	if err := Put("smoke_test", "def456", "passed", Options{Dir: dir, TTL: DefaultTTL}, writtenAt); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	later := writtenAt.Add(DefaultTTL + time.Second)
	got, hit := Get[string]("smoke_test", "def456", Options{Dir: dir, TTL: DefaultTTL}, later)
	if hit {
		t.Fatal("Get() hit = true, want false for an entry older than TTL")
	}
	if got != "" {
		t.Errorf("Get() value = %q, want zero value", got)
	}
}

// TestGet_refreshForcesMissEvenForAFreshEntry proves Refresh short-
// circuits a lookup that would otherwise be a hit. Unlike Lookup, Get
// reads the filesystem directly rather than through an injectable
// interface, so this test cannot spy on call counts the way
// TestLookup_refreshForcesMissWithoutReadingHeader does; instead it
// establishes a baseline hit without Refresh, then shows the same
// entry misses with Refresh set — the observable behavior Get's doc
// comment promises ("Refresh unset" is required for a hit, checked
// before any file access).
func TestGet_refreshForcesMissEvenForAFreshEntry(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	if err := Put("resolved_user", "abc123", int64(42), Options{Dir: dir, TTL: DefaultTTL}, now); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	if _, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL}, now); !hit {
		t.Fatal("baseline Get() hit = false, want true (entry should be fresh without Refresh)")
	}

	got, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL, Refresh: true}, now)
	if hit {
		t.Fatal("Get() hit = true, want false when Refresh is set")
	}
	if got != 0 {
		t.Errorf("Get() value = %d, want zero value", got)
	}
}

func TestGet_absentEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	got, hit := Get[int64]("resolved_user", "missing", Options{Dir: dir, TTL: DefaultTTL}, now)
	if hit {
		t.Fatal("Get() hit = true, want false for an absent entry")
	}
	if got != 0 {
		t.Errorf("Get() value = %d, want zero value", got)
	}
}

func TestGet_malformedEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	path := filepath.Join(dir, valueFileName("resolved_user", "abc123"))
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write fixture file %s: %v", path, err)
	}

	got, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL}, now)
	if hit {
		t.Fatal("Get() hit = true, want false for a malformed entry")
	}
	if got != 0 {
		t.Errorf("Get() value = %d, want zero value", got)
	}
}

// TestGet_unreadableEntryIsMiss arranges a permission-denied read by
// chmod-ing the entry file to 0. This only blocks the read for a
// non-root owner: root (and any process with CAP_DAC_OVERRIDE) bypasses
// file permission bits entirely, so the test skips itself when running
// as root rather than asserting a case it cannot actually arrange.
func TestGet_unreadableEntryIsMiss(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not block root's own reads, so this case cannot be arranged portably here")
	}

	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	if err := Put("resolved_user", "abc123", int64(42), Options{Dir: dir, TTL: DefaultTTL}, now); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	path := filepath.Join(dir, valueFileName("resolved_user", "abc123"))
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}

	got, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL}, now)
	if hit {
		t.Fatal("Get() hit = true, want false for an unreadable entry")
	}
	if got != 0 {
		t.Errorf("Get() value = %d, want zero value", got)
	}
}

// TestValueFileName_neverCollidesWithArtifactNames checks the naming
// scheme's actual guarantee: for every artifact kind/ext combination
// and every value name/hash combination (including a value name equal
// to a real artifact kind, and an empty hash), the two file names never
// match. ExtValue alone is what makes this hold, regardless of name.
func TestValueFileName_neverCollidesWithArtifactNames(t *testing.T) {
	artifactKinds := []string{"mr_list", "comment_list", "labeled_comments", "filtered_comments"}
	artifactExts := []string{ExtYAML, ExtJSON, ExtText, "html", "htm"}
	valueNames := []string{"resolved_user", "smoke_test", "mr_list", "comment_list"}
	hashes := []string{"abc123", "", "abc_123", "0"}

	for _, hash := range hashes {
		for _, valueName := range valueNames {
			valuePath := valueFileName(valueName, hash)
			for _, kind := range artifactKinds {
				for _, ext := range artifactExts {
					artifactPath := FileName(kind, hash, ext)
					if valuePath == artifactPath {
						t.Errorf("valueFileName(%q, %q) = %q collides with FileName(%q, %q, %q)",
							valueName, hash, valuePath, kind, hash, ext)
					}
				}
			}
		}
	}
}

// TestPut_concurrentWritesDoNotCorruptEntry stands in for the
// bogoslav-mcp scenario Put's doc comment describes: several writers
// racing to Put the same name+hash. The atomic rename (temp file +
// os.Rename) means the result must be exactly one well-formed entry
// carrying one of the writers' values, and no leftover temp file — a
// torn write is the failure mode this test would catch.
func TestPut_concurrentWritesDoNotCorruptEntry(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	const writers = 20
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- Put("resolved_user", "abc123", int64(i), Options{Dir: dir, TTL: DefaultTTL}, now)
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Put() error = %v", err)
		}
	}

	got, hit := Get[int64]("resolved_user", "abc123", Options{Dir: dir, TTL: DefaultTTL}, now)
	if !hit {
		t.Fatal("Get() hit = false after concurrent writers, want a clean well-formed entry from one of them")
	}
	if got < 0 || got >= int64(writers) {
		t.Errorf("Get() value = %d, want one of the writers' values in [0, %d)", got, writers)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s) error = %v", dir, err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("directory has %d entries after concurrent writers, want exactly 1 (no leftover temp files): %v", len(entries), names)
	}
}
