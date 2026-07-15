package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

func TestEntryJSON_familyAMatchesDocumentedShape(t *testing.T) {
	d := serverDescriptor{Name: "bogoslav", Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{}}

	got, err := entryJSON(familyA, d)
	if err != nil {
		t.Fatalf("entryJSON: %v", err)
	}

	merged, err := mergeStdioServer(nil, "mcpServers", d.Name, got)
	if err != nil {
		t.Fatalf("mergeStdioServer: %v", err)
	}

	want := `{"mcpServers":{"bogoslav":{"command":"/path/to/bogoslav-mcp","args":[],"env":{}}}}`
	assertSameJSON(t, merged, []byte(want))

	// TZ.md section 9.2: family A never carries a "type" key for a local
	// stdio server.
	if bytes.Contains(merged, []byte(`"type"`)) {
		t.Errorf("family A entry must not have a \"type\" key, got %s", merged)
	}
}

func TestEntryJSON_familyBMatchesDocumentedShape(t *testing.T) {
	d := serverDescriptor{Name: "bogoslav", Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{}}

	got, err := entryJSON(familyB, d)
	if err != nil {
		t.Fatalf("entryJSON: %v", err)
	}

	merged, err := mergeStdioServer(nil, "mcp", d.Name, got)
	if err != nil {
		t.Fatalf("mergeStdioServer: %v", err)
	}

	want := `{"mcp":{"bogoslav":{"type":"local","command":["/path/to/bogoslav-mcp"],"enabled":true,"environment":{}}}}`
	assertSameJSON(t, merged, []byte(want))
}

func TestEntryJSON_familyBCommandIsOneArrayNotCommandPlusArgs(t *testing.T) {
	d := serverDescriptor{Name: "bogoslav", Command: "bogoslav-mcp", Args: []string{"--flag", "value"}, Env: map[string]string{}}

	got, err := entryJSON(familyB, d)
	if err != nil {
		t.Fatalf("entryJSON: %v", err)
	}

	var decoded struct {
		Command []string `json:"command"`
	}
	decodeJSON(t, got, &decoded)

	want := []string{"bogoslav-mcp", "--flag", "value"}
	if len(decoded.Command) != len(want) {
		t.Fatalf("command = %v, want %v", decoded.Command, want)
	}
	for i := range want {
		if decoded.Command[i] != want[i] {
			t.Fatalf("command = %v, want %v", decoded.Command, want)
		}
	}
}

const kiloFixtureWithComments = `{
  // user's own comment above another server
  "mcp": {
    "other-server": {
      "type": "local",
      "command": ["node", "server.js"],
      "enabled": true,
      "environment": {}
    } // trailing comment on other-server
  },
  "theme": "dark", // unrelated top-level setting
}
`

// TestMergeStdioServer_kiloJSONCPreservesComments is the golden test for
// TZ.md's acceptance criterion 12.15 and section 9.3.1's JSONC trap:
// merging into kilo.jsonc must not eat the user's comments.
func TestMergeStdioServer_kiloJSONCPreservesComments(t *testing.T) {
	entry := mustEntryJSON(t, familyB, serverDescriptor{
		Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	out, err := mergeStdioServer([]byte(kiloFixtureWithComments), "mcp", "bogoslav", entry)
	if err != nil {
		t.Fatalf("mergeStdioServer: %v", err)
	}

	for _, comment := range []string{
		"// user's own comment above another server",
		"// trailing comment on other-server",
		"// unrelated top-level setting",
	} {
		if !bytes.Contains(out, []byte(comment)) {
			t.Errorf("merged output lost comment %q; got:\n%s", comment, out)
		}
	}
}

// TestMergeStdioServer_otherEntriesSurviveByteIdentical is the other half
// of TZ.md section 9.3.2's "merged, never overwritten" requirement:
// another MCP server already in the file must come out with the exact
// same command/args/env it went in with -- not just "similar", not
// reformatted into a different shape.
func TestMergeStdioServer_otherEntriesSurviveByteIdentical(t *testing.T) {
	entry := mustEntryJSON(t, familyB, serverDescriptor{
		Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	out, err := mergeStdioServer([]byte(kiloFixtureWithComments), "mcp", "bogoslav", entry)
	if err != nil {
		t.Fatalf("mergeStdioServer: %v", err)
	}

	var before, after struct {
		MCP map[string]struct {
			Type        string            `json:"type"`
			Command     []string          `json:"command"`
			Enabled     bool              `json:"enabled"`
			Environment map[string]string `json:"environment"`
		} `json:"mcp"`
		Theme string `json:"theme"`
	}
	decodeJSON(t, standardizeJSONC(t, kiloFixtureWithComments), &before)
	decodeJSON(t, standardizeJSONC(t, string(out)), &after)

	wantOther := before.MCP["other-server"]
	gotOther, ok := after.MCP["other-server"]
	if !ok {
		t.Fatalf("other-server entry disappeared from merged output:\n%s", out)
	}
	if gotOther.Type != wantOther.Type ||
		len(gotOther.Command) != len(wantOther.Command) ||
		gotOther.Command[0] != wantOther.Command[0] || gotOther.Command[1] != wantOther.Command[1] ||
		gotOther.Enabled != wantOther.Enabled {
		t.Fatalf("other-server changed: got %+v, want %+v", gotOther, wantOther)
	}
	if after.Theme != before.Theme {
		t.Fatalf("unrelated top-level key changed: got %q, want %q", after.Theme, before.Theme)
	}
}

// TestMergeStdioServer_installingTwiceIsIdempotent covers TZ.md section
// 9.3's "installing twice must be idempotent": the second merge with the
// exact same entry must produce byte-identical output to the first, not
// a second "bogoslav" member or any other duplication.
func TestMergeStdioServer_installingTwiceIsIdempotent(t *testing.T) {
	entry := mustEntryJSON(t, familyB, serverDescriptor{
		Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	first, err := mergeStdioServer([]byte(kiloFixtureWithComments), "mcp", "bogoslav", entry)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	second, err := mergeStdioServer(first, "mcp", "bogoslav", entry)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatalf("second merge is not byte-identical to the first:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if n := strings.Count(string(second), `"bogoslav"`); n != 1 {
		t.Fatalf(`"bogoslav" appears %d times after two merges, want exactly 1:\n%s`, n, second)
	}
}

// TestMergeStdioServer_reinstallWithDifferentCommandReplacesInPlace
// checks the other side of idempotency: re-running install with a
// genuinely different descriptor (the user moved the bogoslav-mcp
// binary) replaces the one entry's value rather than leaving a stale
// duplicate or refusing to update.
func TestMergeStdioServer_reinstallWithDifferentCommandReplacesInPlace(t *testing.T) {
	first := mustEntryJSON(t, familyB, serverDescriptor{
		Command: "/old/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})
	second := mustEntryJSON(t, familyB, serverDescriptor{
		Command: "/new/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	out, err := mergeStdioServer([]byte(kiloFixtureWithComments), "mcp", "bogoslav", first)
	if err != nil {
		t.Fatalf("first merge: %v", err)
	}
	out, err = mergeStdioServer(out, "mcp", "bogoslav", second)
	if err != nil {
		t.Fatalf("second merge: %v", err)
	}

	if bytes.Contains(out, []byte("/old/path/to/bogoslav-mcp")) {
		t.Errorf("stale command survived reinstall:\n%s", out)
	}
	if !bytes.Contains(out, []byte("/new/path/to/bogoslav-mcp")) {
		t.Errorf("new command missing after reinstall:\n%s", out)
	}
	if n := strings.Count(string(out), `"bogoslav"`); n != 1 {
		t.Fatalf(`"bogoslav" appears %d times after reinstall, want exactly 1:\n%s`, n, out)
	}
}

func TestMergeStdioServer_createsParentObjectWhenMissing(t *testing.T) {
	entry := mustEntryJSON(t, familyA, serverDescriptor{
		Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	out, err := mergeStdioServer([]byte(`{"unrelated": true}`), "mcpServers", "bogoslav", entry)
	if err != nil {
		t.Fatalf("mergeStdioServer: %v", err)
	}

	var decoded struct {
		Unrelated  bool `json:"unrelated"`
		MCPServers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	decodeJSON(t, out, &decoded)

	if !decoded.Unrelated {
		t.Errorf("unrelated pre-existing key lost: %s", out)
	}
	if decoded.MCPServers["bogoslav"].Command != "/path/to/bogoslav-mcp" {
		t.Errorf("bogoslav entry missing or wrong: %s", out)
	}
}

func TestMergeStdioServer_emptyExistingFileIsTreatedAsEmptyObject(t *testing.T) {
	entry := mustEntryJSON(t, familyA, serverDescriptor{
		Command: "/path/to/bogoslav-mcp", Args: []string{}, Env: map[string]string{},
	})

	out, err := mergeStdioServer(nil, "mcpServers", "bogoslav", entry)
	if err != nil {
		t.Fatalf("mergeStdioServer on nil existing: %v", err)
	}
	if !bytes.Contains(out, []byte("bogoslav")) {
		t.Errorf("expected a fresh config to contain the new entry, got %s", out)
	}
}

func TestMergeStdioServer_rejectsNonObjectRoot(t *testing.T) {
	entry := mustEntryJSON(t, familyA, serverDescriptor{Command: "x", Args: []string{}, Env: map[string]string{}})

	if _, err := mergeStdioServer([]byte(`[1,2,3]`), "mcpServers", "bogoslav", entry); err == nil {
		t.Fatal("expected an error merging into an array root, got nil")
	}
}

func mustEntryJSON(t *testing.T, f family, d serverDescriptor) []byte {
	t.Helper()
	got, err := entryJSON(f, d)
	if err != nil {
		t.Fatalf("entryJSON: %v", err)
	}
	return got
}

// standardizeJSONC strips JWCC-only syntax (comments, trailing commas)
// from jsonc so tests can decode a JSONC fixture with the standard
// library's encoding/json, which does not accept either.
func standardizeJSONC(t *testing.T, jsonc string) []byte {
	t.Helper()
	out, err := hujson.Standardize([]byte(jsonc))
	if err != nil {
		t.Fatalf("standardize jsonc fixture: %v", err)
	}
	return out
}
