package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

// readTaxonomyFile reads a custom taxonomy from a JSON file (TZ.md
// section 8.5: "taxonomy_version -- целое, пользователь может её
// редактировать"), returning nil when path is empty so the caller's
// request field stays nil and app falls back to classify.DefaultTaxonomy.
// classify.Taxonomy's own UnmarshalJSON rejects a set missing the
// required "other" fallback label, so that check is not repeated here.
func readTaxonomyFile(path string) (*classify.Taxonomy, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read --taxonomy-file %q: %w", path, err)
	}
	var t classify.Taxonomy
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse --taxonomy-file %q: %w", path, err)
	}
	return &t, nil
}

// readNoteLabels reads the labeling result save-labels validates and
// writes (TZ.md section 8.1): a JSON array of {"note_id": ..., "label":
// ...} entries, from path, or from stdin when path is "-".
func readNoteLabels(cmd *cobra.Command, path string) ([]classify.NoteLabel, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(cmd.InOrStdin())
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read --labels %q: %w", path, err)
	}

	var labels []classify.NoteLabel
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("parse --labels %q: %w", path, err)
	}
	return labels, nil
}
