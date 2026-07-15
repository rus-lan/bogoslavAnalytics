package cache

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeHeaderReader stubs the artifact provenance header read that
// Lookup delegates through the HeaderReader interface, without cache
// depending on artifact/'s yaml/json parsing.
type fakeHeaderReader struct {
	fetchedAt map[string]time.Time
	calls     []string
}

func (f *fakeHeaderReader) FetchedAt(path string) (time.Time, error) {
	f.calls = append(f.calls, path)
	t, ok := f.fetchedAt[path]
	if !ok {
		return time.Time{}, errors.New("fake header reader: no fetched_at recorded for " + path)
	}
	return t, nil
}

func writeEmptyFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write fixture file %s: %v", path, err)
	}
}

func TestLookup_freshEntryIsHit(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	path := filepath.Join(dir, FileName("mr_list", "abc123", ExtYAML))
	writeEmptyFile(t, path)

	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		path: now.Add(-1 * time.Hour),
	}}

	got, hit, err := Lookup("mr_list", "abc123", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !hit {
		t.Fatal("Lookup() hit = false, want true")
	}
	if got != path {
		t.Errorf("Lookup() path = %q, want %q", got, path)
	}
}

func TestLookup_expiredEntryIsMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	path := filepath.Join(dir, FileName("mr_list", "abc123", ExtYAML))
	writeEmptyFile(t, path)

	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		path: now.Add(-25 * time.Hour),
	}}

	got, hit, err := Lookup("mr_list", "abc123", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if hit {
		t.Fatal("Lookup() hit = true, want false")
	}
	if got != "" {
		t.Errorf("Lookup() path = %q, want empty", got)
	}
}

func TestLookup_refreshForcesMissWithoutReadingHeader(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	path := filepath.Join(dir, FileName("mr_list", "abc123", ExtYAML))
	writeEmptyFile(t, path)

	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		path: now.Add(-1 * time.Minute), // would be fresh, if refresh did not skip it
	}}

	got, hit, err := Lookup("mr_list", "abc123", Options{Dir: dir, TTL: DefaultTTL, Refresh: true}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if hit {
		t.Fatal("Lookup() hit = true, want false when Refresh is set")
	}
	if got != "" {
		t.Errorf("Lookup() path = %q, want empty", got)
	}
	if len(headers.calls) != 0 {
		t.Errorf("Lookup() read the header %d time(s) despite Refresh, want 0", len(headers.calls))
	}
}

func TestLookup_textFileNeverHits(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	// Only a .txt artifact exists for this kind+hash — no yaml/json.
	textPath := filepath.Join(dir, FileName("comment_list", "abc123", ExtText))
	writeEmptyFile(t, textPath)

	// Even if the header reader would happily report the .txt file as
	// fresh, Lookup must never ask about it: text is write-only.
	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		textPath: now,
	}}

	got, hit, err := Lookup("comment_list", "abc123", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if hit {
		t.Fatal("Lookup() hit = true for a .txt-only artifact, want false")
	}
	if got != "" {
		t.Errorf("Lookup() path = %q, want empty", got)
	}
	for _, call := range headers.calls {
		if call == textPath {
			t.Errorf("Lookup() read the header of a .txt file %q, want it never checked", textPath)
		}
	}
}

func TestLookup_textFileIgnoredEvenWithExpiredYAMLPresent(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	yamlPath := filepath.Join(dir, FileName("comment_list", "abc123", ExtYAML))
	writeEmptyFile(t, yamlPath)
	textPath := filepath.Join(dir, FileName("comment_list", "abc123", ExtText))
	writeEmptyFile(t, textPath)

	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		yamlPath: now.Add(-25 * time.Hour), // expired
		textPath: now,                      // fresh, but must be ignored
	}}

	got, hit, err := Lookup("comment_list", "abc123", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if hit {
		t.Fatal("Lookup() hit = true, want false: the only non-text artifact is expired")
	}
	if got != "" {
		t.Errorf("Lookup() path = %q, want empty", got)
	}
}

func TestLookup_noFileIsMiss(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{}}

	got, hit, err := Lookup("mr_list", "missing", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if hit {
		t.Fatal("Lookup() hit = true, want false")
	}
	if got != "" {
		t.Errorf("Lookup() path = %q, want empty", got)
	}
}

func TestLookup_jsonUsedWhenYAMLAbsent(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	jsonPath := filepath.Join(dir, FileName("mr_list", "abc123", ExtJSON))
	writeEmptyFile(t, jsonPath)

	headers := &fakeHeaderReader{fetchedAt: map[string]time.Time{
		jsonPath: now.Add(-1 * time.Hour),
	}}

	got, hit, err := Lookup("mr_list", "abc123", Options{Dir: dir, TTL: DefaultTTL}, headers, now)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if !hit {
		t.Fatal("Lookup() hit = false, want true")
	}
	if got != jsonPath {
		t.Errorf("Lookup() path = %q, want %q", got, jsonPath)
	}
}
