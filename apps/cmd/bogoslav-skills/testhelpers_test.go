package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// decodeJSON unmarshals data into v, failing the test on error. It works
// on plain JSON output (hujson.Value.Pack's output is always valid plain
// JSON when nothing in the source used JSONC-only syntax).
func decodeJSON(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode json: %v\ndata: %s", err, data)
	}
}

// assertSameJSON compares got and want as JSON values (key order and
// whitespace do not matter), failing the test with both renderings if
// they differ.
func assertSameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var gotVal, wantVal any
	decodeJSON(t, got, &gotVal)
	decodeJSON(t, want, &wantVal)
	if !reflect.DeepEqual(gotVal, wantVal) {
		t.Fatalf("json mismatch:\ngot:  %s\nwant: %s", got, want)
	}
}

// writeTestFile writes content to path, creating any missing parent
// directory, failing the test on error.
func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create directory for %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
