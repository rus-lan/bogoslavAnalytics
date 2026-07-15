package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tailscale/hujson"
)

// familyAEntry is one mcpServers entry (TZ.md section 9.2): claude's
// .mcp.json, cursor's .cursor/mcp.json, cline's ~/.cline/mcp.json. There
// is no "type" key: the research fixture for this family never carries
// one for a local stdio server.
type familyAEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// familyBEntry is one mcp entry (TZ.md section 9.2): opencode's
// opencode.json/.jsonc, kilo's kilo.jsonc/.kilo/kilo.jsonc. command is
// ONE array (the binary followed by its arguments, not a separate
// command+args pair) and the environment key is "environment", not "env"
// -- both deliberate departures from family A that a naive shared struct
// would get wrong.
type familyBEntry struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command"`
	Enabled     bool              `json:"enabled"`
	Environment map[string]string `json:"environment"`
}

// entryJSON renders d as the wire shape for f, 2-space indented so a
// freshly created config file (or a freshly created server entry inside
// an existing one) is human-readable. It never marshals a nil slice or
// map: newDescriptor always hands back non-nil Args/Env, so this always
// emits "[]"/"{}", never "null".
func entryJSON(f family, d serverDescriptor) ([]byte, error) {
	switch f {
	case familyA:
		return json.MarshalIndent(familyAEntry{
			Command: d.Command,
			Args:    d.Args,
			Env:     d.Env,
		}, "", "  ")
	case familyB:
		command := make([]string, 0, 1+len(d.Args))
		command = append(command, d.Command)
		command = append(command, d.Args...)
		return json.MarshalIndent(familyBEntry{
			Type:        "local",
			Command:     command,
			Enabled:     true,
			Environment: d.Env,
		}, "", "  ")
	default:
		return nil, fmt.Errorf("mcpconfig: unknown family %d", f)
	}
}

// mergeStdioServer upserts a JSON object member at /parentKey/name into
// existing, returning the whole file's new bytes. It is the one place
// that touches a target's config file's content, and it is deliberately
// narrow: TZ.md section 9.3.1 requires that an existing config's other
// MCP servers, comments, and formatting survive a merge untouched, and
// the surest way to guarantee that is to never rewrite anything this
// command was not asked to change.
//
// existing may be nil or empty (no config file yet): it is then treated
// as an empty JSON object. The file may be plain JSON or JSONC (comments
// and trailing commas, per the JWCC extension) -- both parse the same way
// through hujson, since "All JSON is valid JWCC".
//
// mergeStdioServer uses hujson's Patch method (RFC 6902 JSON Patch)
// rather than Parse-mutate-Format: Patch's insertAt/replaceAt machinery
// (see the package's own patch.go) is what correctly keeps a comment that
// sits after the previously-last member attached to that member rather
// than sliding it onto whatever is inserted after it -- a real failure
// mode this command's own experiments hit when the member list was edited
// by hand instead. Format is deliberately never called afterward: Format
// re-indents and re-aligns the whole document, which would violate the
// "other entries survive byte-identical" requirement it is not worth
// trading away for prettier alignment of the one entry this command owns.
func mergeStdioServer(existing []byte, parentKey, name string, entry []byte) ([]byte, error) {
	if len(bytes.TrimSpace(existing)) == 0 {
		existing = []byte("{}\n")
	}

	root, err := hujson.Parse(existing)
	if err != nil {
		return nil, fmt.Errorf("mcpconfig: parse existing config: %w", err)
	}
	if _, ok := root.Value.(*hujson.Object); !ok {
		return nil, fmt.Errorf("mcpconfig: config root is not a JSON object")
	}

	parentPointer := "/" + jsonPointerEscape(parentKey)
	if root.Find(parentPointer) == nil {
		patch, err := addOperation(parentPointer, []byte("{}"))
		if err != nil {
			return nil, err
		}
		if err := root.Patch(patch); err != nil {
			return nil, fmt.Errorf("mcpconfig: create %q object: %w", parentKey, err)
		}
	}

	entryPointer := parentPointer + "/" + jsonPointerEscape(name)
	patch, err := addOperation(entryPointer, entry)
	if err != nil {
		return nil, err
	}
	if err := root.Patch(patch); err != nil {
		return nil, fmt.Errorf("mcpconfig: write %q entry: %w", name, err)
	}

	return root.Pack(), nil
}

// addOperation builds a one-operation RFC 6902 JSON Patch document that
// adds (or, if the member already exists, replaces -- per RFC 6902's own
// "add" semantics) value at pointer.
func addOperation(pointer string, value []byte) ([]byte, error) {
	path, err := json.Marshal(pointer)
	if err != nil {
		return nil, fmt.Errorf("mcpconfig: marshal json pointer %q: %w", pointer, err)
	}
	var buf bytes.Buffer
	buf.WriteString(`[{"op":"add","path":`)
	buf.Write(path)
	buf.WriteString(`,"value":`)
	buf.Write(bytes.TrimSpace(value))
	buf.WriteString(`}]`)
	return buf.Bytes(), nil
}

// jsonPointerEscape escapes seg per RFC 6901 section 3 so it can be used
// as one segment of a JSON Pointer: "~" first (to "~0"), then "/" (to
// "~1"). None of this command's own key names ("mcpServers", "mcp",
// "bogoslav") need it, but the config's other, user-chosen server names
// might in principle contain either character.
func jsonPointerEscape(seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	seg = strings.ReplaceAll(seg, "/", "~1")
	return seg
}
