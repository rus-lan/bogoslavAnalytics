package artifact

import (
	"fmt"
	"time"
)

// HeaderStore reads just the header of an artifact file — its
// schema_version and source — without needing the item arrays the
// rest of the document carries. A *HeaderStore satisfies the cache
// package's consumer-side HeaderReader interface structurally (TZ.md
// sections 2.4 and 4.6): artifact never imports cache, so the
// compile-time proof that the two line up lives in
// header_store_test.go, which is free to import cache.
type HeaderStore struct{}

// FetchedAt reads the header of the artifact at path and returns its
// source.fetched_at timestamp. Header is common to all four artifact
// kinds, so this works regardless of which kind path holds.
//
// A write-only path (text or html) fails with ErrNotReadable, via
// decodeFile's existing Format.writable() check; a missing or
// malformed file fails with the wrapped os/json/yaml error decodeFile
// already produces; an unknown schema_version fails with
// ErrUnknownSchemaVersion. None of these ever pair a zero time with a
// nil error, so a caller can never mistake "could not read the
// header" for "found, but stale forever" — or for a fresh hit.
//
// Decoding goes through the same decodeFile used by the four ReadX
// functions rather than a hand-rolled partial scanner: for json this
// already skips over the item arrays, since json.Unmarshal only
// allocates the fields present on Header; for yaml, decodeFile's
// generic-tree pipeline (see unmarshalYAML) still walks the whole
// document, so there is no equivalent saving there. That is judged an
// acceptable simplification over hand-rolling a YAML scanner.
func (*HeaderStore) FetchedAt(path string) (time.Time, error) {
	var h Header
	if err := decodeFile(path, &h); err != nil {
		return time.Time{}, err
	}
	if h.SchemaVersion != CurrentSchemaVersion {
		return time.Time{}, fmt.Errorf("read %q: schema_version %d: %w", path, h.SchemaVersion, ErrUnknownSchemaVersion)
	}
	return h.Source.FetchedAt, nil
}
