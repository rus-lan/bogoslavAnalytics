package artifact

import "time"

// CurrentSchemaVersion is the schema_version this package writes on
// every artifact. Reading an artifact written with a different version
// fails with ErrUnknownSchemaVersion (TZ.md section 4).
const CurrentSchemaVersion = 1

// Source records where an artifact's data came from: the GitLab
// instance it was fetched from and when (TZ.md section 4).
type Source struct {
	GitlabURL string    `json:"gitlab_url"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Header is the set of fields common to every artifact kind: the schema
// version, the artifact kind, and its source. It is embedded in each of
// the four artifact document types (TZ.md section 4).
type Header struct {
	SchemaVersion int    `json:"schema_version"`
	Kind          Kind   `json:"kind"`
	Source        Source `json:"source"`
}
