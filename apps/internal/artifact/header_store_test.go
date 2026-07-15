package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/cache"
)

// var _ cache.HeaderReader = (*HeaderStore)(nil) is the compile-time
// proof that *HeaderStore structurally satisfies cache.HeaderReader
// (TZ.md sections 2.4 and 4.6). It lives here, in a test file, so
// that artifact itself never imports cache: cache does not import
// artifact either, so this does not create an import cycle.
var _ cache.HeaderReader = (*HeaderStore)(nil)

// TestFetchedAt_roundTripsAcrossFormatsAndKinds checks that
// HeaderStore.FetchedAt returns exactly the source.fetched_at that
// was written, for both readable formats (json, yaml) and for all
// four artifact kinds — Header is common to each of them, and every
// sample fixture shares the same fetched_at value.
func TestFetchedAt_roundTripsAcrossFormatsAndKinds(t *testing.T) {
	want := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	store := &HeaderStore{}

	for _, k := range allKindCases() {
		for _, format := range []Format{FormatJSON, FormatYAML} {
			t.Run(k.name+"/"+string(format), func(t *testing.T) {
				ext, err := format.Extension()
				if err != nil {
					t.Fatalf("Extension() error = %v", err)
				}
				path := filepath.Join(t.TempDir(), k.name+"_header_test."+ext)

				if err := k.write(format, path); err != nil {
					t.Fatalf("write() error = %v", err)
				}

				got, err := store.FetchedAt(path)
				if err != nil {
					t.Fatalf("FetchedAt() error = %v", err)
				}
				if !got.Equal(want) {
					t.Errorf("FetchedAt() = %v, want %v", got, want)
				}
			})
		}
	}
}

// TestFetchedAt_txtIsNotReadable checks that a .txt artifact path
// fails with ErrNotReadable rather than a parsed time: text is
// write-only (TZ.md section 4).
func TestFetchedAt_txtIsNotReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mr_list_test.txt")
	if err := WriteMRList(sampleMRList(), FormatText, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	store := &HeaderStore{}
	got, err := store.FetchedAt(path)
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("FetchedAt() error = %v, want ErrNotReadable", err)
	}
	if !got.IsZero() {
		t.Errorf("FetchedAt() time = %v, want zero value alongside ErrNotReadable", got)
	}
}

// TestFetchedAt_htmlIsNotReadable is the html twin of
// TestFetchedAt_txtIsNotReadable: html is also write-only.
func TestFetchedAt_htmlIsNotReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mr_list_test.html")
	if err := WriteMRList(sampleMRList(), FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	store := &HeaderStore{}
	got, err := store.FetchedAt(path)
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("FetchedAt() error = %v, want ErrNotReadable", err)
	}
	if !got.IsZero() {
		t.Errorf("FetchedAt() time = %v, want zero value alongside ErrNotReadable", got)
	}
}

// TestFetchedAt_missingFileIsWrappedError checks that a path with no
// file on disk fails with a wrapped os.ErrNotExist, not a zero time
// alongside a nil error — a caller can tell "missing" apart from
// "found, but stale".
func TestFetchedAt_missingFileIsWrappedError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does_not_exist.json")

	store := &HeaderStore{}
	got, err := store.FetchedAt(path)
	if err == nil {
		t.Fatal("FetchedAt() error = nil, want a wrapped not-exist error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("FetchedAt() error = %v, want it to wrap os.ErrNotExist", err)
	}
	if !got.IsZero() {
		t.Errorf("FetchedAt() time = %v, want zero value alongside a non-nil error", got)
	}
}

// TestFetchedAt_malformedFileIsWrappedError checks that a file which
// exists but does not parse as the declared format fails with a
// wrapped decode error, for both readable formats.
func TestFetchedAt_malformedFileIsWrappedError(t *testing.T) {
	cases := []struct {
		name string
		ext  string
		data string
	}{
		{"json", "json", "{ this is not valid json"},
		// yaml.v3 rejects a tab used for indentation: a reliable,
		// unambiguous parse failure.
		{"yaml", "yaml", "schema_version: 1\n\tkind: mr_list\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "malformed_header_test."+tc.ext)
			if err := os.WriteFile(path, []byte(tc.data), 0o644); err != nil {
				t.Fatalf("write fixture %q: %v", path, err)
			}

			store := &HeaderStore{}
			got, err := store.FetchedAt(path)
			if err == nil {
				t.Fatal("FetchedAt() error = nil, want a wrapped decode error")
			}
			if errors.Is(err, ErrUnknownSchemaVersion) {
				t.Errorf("FetchedAt() error = %v, want a decode error, not ErrUnknownSchemaVersion", err)
			}
			if !got.IsZero() {
				t.Errorf("FetchedAt() time = %v, want zero value alongside a non-nil error", got)
			}
		})
	}
}

// TestFetchedAt_unknownSchemaVersionIsWrappedError checks that a
// well-formed file whose schema_version does not match
// CurrentSchemaVersion fails with ErrUnknownSchemaVersion, for both
// readable formats.
func TestFetchedAt_unknownSchemaVersionIsWrappedError(t *testing.T) {
	for _, format := range []Format{FormatJSON, FormatYAML} {
		t.Run(string(format), func(t *testing.T) {
			ext, err := format.Extension()
			if err != nil {
				t.Fatalf("Extension() error = %v", err)
			}
			path := filepath.Join(t.TempDir(), "mr_list_header_test."+ext)

			if err := WriteMRList(sampleMRList(), format, path); err != nil {
				t.Fatalf("WriteMRList() error = %v", err)
			}
			tamperSchemaVersion(t, path, format)

			store := &HeaderStore{}
			got, err := store.FetchedAt(path)
			if !errors.Is(err, ErrUnknownSchemaVersion) {
				t.Errorf("FetchedAt() error = %v, want ErrUnknownSchemaVersion", err)
			}
			if !got.IsZero() {
				t.Errorf("FetchedAt() time = %v, want zero value alongside ErrUnknownSchemaVersion", got)
			}
		})
	}
}
