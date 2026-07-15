package artifact

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Format is one of the three artifact wire formats (TZ.md section 4).
type Format string

const (
	FormatJSON Format = "json"
	FormatYAML Format = "yaml"
	// FormatText and FormatHTML are write-only: neither is cacheable,
	// and neither can ever be read back as a from_artifact input
	// (TZ.md section 4). FormatText renders a plain, comment-headed
	// text file; FormatHTML renders a self-contained, styled report
	// page.
	FormatText Format = "text"
	FormatHTML Format = "html"
)

// writable reports whether a decoded artifact can be read back from a
// file written in this format. Only json and yaml round-trip; text and
// html are presentation-only and share the one write-only rule.
func (f Format) writable() bool {
	return f == FormatJSON || f == FormatYAML
}

// Extension returns the file extension used to write a file in this
// format, without the leading dot.
func (f Format) Extension() (string, error) {
	switch f {
	case FormatJSON:
		return "json", nil
	case FormatYAML:
		return "yaml", nil
	case FormatText:
		return "txt", nil
	case FormatHTML:
		return "html", nil
	default:
		return "", fmt.Errorf("format %q: %w", f, ErrUnsupportedFormat)
	}
}

// FormatFromPath infers the artifact format from a file path's
// extension: ".json" is FormatJSON, ".yaml"/".yml" is FormatYAML,
// ".txt" is FormatText, and ".html"/".htm" is FormatHTML. Any other
// extension is ErrUnsupportedFormat.
func FormatFromPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		return FormatJSON, nil
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".txt":
		return FormatText, nil
	case ".html", ".htm":
		return FormatHTML, nil
	default:
		return "", fmt.Errorf("path %q: %w", path, ErrUnsupportedFormat)
	}
}
