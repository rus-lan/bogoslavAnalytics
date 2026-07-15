package cache

// Artifact file extensions (TZ.md section 4).
const (
	ExtYAML = "yaml"
	ExtJSON = "json"
	// ExtText is write-only: TZ.md section 4.5 states a cache lookup
	// ignores .txt files, so Lookup never checks this extension.
	ExtText = "txt"
)

// FileName builds the artifact file name "<kind>_<hash>.<ext>"
// (TZ.md section 4.5).
func FileName(kind, hash, ext string) string {
	return kind + "_" + hash + "." + ext
}
