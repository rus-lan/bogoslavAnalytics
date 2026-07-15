package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// marshalJSON renders v as indented JSON, the wire format for
// FormatJSON.
func marshalJSON(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return append(b, '\n'), nil
}

// unmarshalJSON parses JSON bytes into v.
func unmarshalJSON(data []byte, v any) error {
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("unmarshal json: %w", err)
	}
	return nil
}

// marshalYAML renders v as YAML. The domain types this package embeds
// (Query, Note, Date, Classifier, ...) carry only json struct tags, so
// this does not call yaml.Marshal(v) directly: it marshals v to JSON
// first (the single source of truth for field names and for the custom
// codecs on types like domain.Date), parses that JSON as a YAML node
// tree (JSON is valid YAML flow syntax), clears the flow styling picked
// up from the JSON source so the result renders in normal YAML block
// style, and marshals the node tree.
func marshalYAML(v any) ([]byte, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: convert to json: %w", err)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(jsonBytes, &node); err != nil {
		return nil, fmt.Errorf("marshal yaml: parse json as yaml: %w", err)
	}
	clearFlowStyle(&node)

	out, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("marshal yaml: %w", err)
	}
	return out, nil
}

// clearFlowStyle resets the style of a node and every descendant to the
// default (block) style, recursively.
func clearFlowStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		clearFlowStyle(c)
	}
}

// unmarshalYAML parses YAML bytes into v. It decodes the YAML into a
// generic value, re-encodes that value as JSON, and unmarshals the JSON
// into v — the mirror image of marshalYAML, so that reading goes
// through the same json struct tags and custom json codecs (domain.Date,
// domain.NoteType, ...) used to write.
func unmarshalYAML(data []byte, v any) error {
	var generic any
	if err := yaml.Unmarshal(data, &generic); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}

	jsonBytes, err := json.Marshal(generic)
	if err != nil {
		return fmt.Errorf("unmarshal yaml: convert to json: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, v); err != nil {
		return fmt.Errorf("unmarshal yaml: %w", err)
	}
	return nil
}

// encode renders v in the given format. FormatText is not handled here:
// callers render text separately, since it needs a human-readable
// comment header rather than a lossless encoding.
func encode(v any, format Format) ([]byte, error) {
	switch format {
	case FormatJSON:
		return marshalJSON(v)
	case FormatYAML:
		return marshalYAML(v)
	default:
		return nil, fmt.Errorf("encode: %w: %q", ErrUnsupportedFormat, format)
	}
}

// artifactFileMode is the permission mode artifact files are written
// with: owner read/write only. Artifacts carry review comment bodies
// together with commenters' real names, usernames and numeric ids
// (TZ.md section 4), so a shared host must not be able to read them
// through group or other bits.
const artifactFileMode = 0o600

// artifactDirMode is the permission mode a newly created artifact
// directory is given: owner access only. The files inside are already
// unreadable to another local user at artifactFileMode, but a wider
// directory mode would still let that user list the artifact
// filenames (each is "<kind>_<hash>.<ext>", so the listing itself
// carries no personal data, but there is no reason to allow even
// that).
const artifactDirMode = 0o700

// writeFile writes data to path, creating any missing parent
// directories.
//
// os.WriteFile's mode argument only applies to a file it creates; an
// existing file keeps whatever mode it already has. So that an
// artifact written before this package tightened its default (or
// otherwise sitting at a looser mode) is not left there forever,
// every write ends with an explicit chmod to artifactFileMode rather
// than only setting it on first creation.
//
// The parent directory gets the same "at creation only" treatment,
// deliberately not extended with a matching chmod: os.MkdirAll is a
// no-op on a directory that already exists, so a pre-existing
// directory keeps its current mode. Unlike file contents, that is not
// a data leak (see artifactDirMode), so retroactively tightening it
// is not worth the risk of overriding a mode set for some other
// reason.
func writeFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, artifactDirMode); err != nil {
		return fmt.Errorf("write file: create directory %q: %w", dir, err)
	}
	if err := os.WriteFile(path, data, artifactFileMode); err != nil {
		return fmt.Errorf("write file %q: %w", path, err)
	}
	if err := os.Chmod(path, artifactFileMode); err != nil {
		return fmt.Errorf("write file %q: set permissions: %w", path, err)
	}
	return nil
}

// decodeFile reads the artifact at path and decodes it into v.
// The format is inferred from the file extension. Reading a write-only
// format (text or html) fails with ErrNotReadable (TZ.md section 4):
// both are presentation-only and do not round-trip back to structured
// data. This is the single rule write-only formats share; adding a
// future write-only format only requires updating Format.writable.
func decodeFile(path string, v any) error {
	format, err := FormatFromPath(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}
	if !format.writable() {
		return fmt.Errorf("read %q: %w", path, ErrNotReadable)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %q: %w", path, err)
	}

	switch format {
	case FormatJSON:
		if err := unmarshalJSON(data, v); err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
	case FormatYAML:
		if err := unmarshalYAML(data, v); err != nil {
			return fmt.Errorf("read %q: %w", path, err)
		}
	default:
		return fmt.Errorf("read %q: %w: %q", path, ErrUnsupportedFormat, format)
	}
	return nil
}

// checkHeader validates a decoded artifact's header against the schema
// version this package writes and the kind the caller expects.
func checkHeader(path string, h Header, want Kind) error {
	if h.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("read %q: schema_version %d: %w", path, h.SchemaVersion, ErrUnknownSchemaVersion)
	}
	if h.Kind != want {
		return fmt.Errorf("read %q: kind %q, want %q: %w", path, h.Kind, want, ErrKindMismatch)
	}
	return nil
}
