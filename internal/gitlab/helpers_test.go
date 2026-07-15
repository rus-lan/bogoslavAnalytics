package gitlab

import (
	"encoding/json"
	"net/http"
	"testing"
)

// writeJSON encodes v as the response body, failing the test if encoding
// fails.
func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
