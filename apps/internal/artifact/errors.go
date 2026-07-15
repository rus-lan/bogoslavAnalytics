package artifact

import "errors"

// Sentinel errors for known failure modes in the artifact layer.
var (
	// ErrNotReadable is returned whenever code tries to read an artifact
	// back from a write-only format: text or html. Both carry
	// human-readable presentation (a comment header for text, a full
	// page for html) that does not round-trip to structured data; both
	// are never cached and never accepted as a from_artifact input
	// (TZ.md section 4).
	ErrNotReadable = errors.New("artifact: this format is write-only and cannot be read back")

	// ErrUnsupportedFormat is returned for a format value, or a file
	// extension, that is none of json, yaml, text, or html.
	ErrUnsupportedFormat = errors.New("artifact: unsupported format")

	// ErrUnknownSchemaVersion is returned when a decoded artifact's
	// schema_version does not match the version this package writes.
	ErrUnknownSchemaVersion = errors.New("artifact: unknown schema version")

	// ErrKindMismatch is returned when a decoded artifact's kind field
	// does not match the kind the caller asked to read.
	ErrKindMismatch = errors.New("artifact: kind mismatch")

	// ErrMissingClassifier is returned when writing a labeled_comments
	// artifact without a fully populated classifier provenance block
	// (TZ.md section 8.3): tool, model, taxonomy_version and
	// classified_at are all required.
	ErrMissingClassifier = errors.New("artifact: classifier provenance is required")
)
