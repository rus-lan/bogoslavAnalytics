// Package cache provides TTL-based lookup of artifacts keyed by the
// normalized query that produced them (TZ.md section 4.5): a SHA-256
// hash over the canonical JSON encoding of the request, the
// "<kind>_<hash>.<ext>" artifact file name, and a freshness check that
// treats text (.txt) artifacts as write-only and never a cache hit. It
// also provides the separate labeling cache key from TZ.md section 8.4.
//
// cache does not parse the text/yaml/json artifact formats itself —
// that lives in artifact/. Where a lookup needs an existing artifact's
// provenance header, cache defines a small consumer-side interface
// (HeaderReader) instead of importing artifact/, per TZ.md section 2.4.
//
// Beyond artifacts, cache also stores small keyed scalars that are not
// artifacts at all and carry no schema_version/kind/source header of
// their own — a resolved username -> numeric id, a smoke-test result
// (TZ.md sections 5.0, 5.5). Get and Put (value.go) persist and read
// these back the same way Lookup does for artifacts (same Options, same
// TTL/Refresh rules), under a reserved file extension (ExtValue) that
// never collides with an artifact file name.
package cache
