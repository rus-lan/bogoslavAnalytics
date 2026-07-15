package classify

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestNewTaxonomy_labelSet(t *testing.T) {
	cases := []struct {
		name    string
		version int
		labels  []string
		wantErr error
	}{
		{
			name:    "includes other",
			version: 1,
			labels:  []string{"bug", "style", "other"},
		},
		{
			name:    "missing other",
			version: 1,
			labels:  []string{"bug", "style"},
			wantErr: ErrTaxonomyMissingOther,
		},
		{
			name:    "empty set",
			version: 1,
			labels:  nil,
			wantErr: ErrTaxonomyMissingOther,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewTaxonomy(tc.version, tc.labels)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewTaxonomy() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewTaxonomy() error = %v", err)
			}
			if got.Version != tc.version {
				t.Errorf("Version = %d, want %d", got.Version, tc.version)
			}
			if !got.Has(LabelOther) {
				t.Errorf("Has(other) = false, want true")
			}
		})
	}
}

func TestDefaultTaxonomy_isValidV1Set(t *testing.T) {
	tax := DefaultTaxonomy()

	if tax.Version != DefaultTaxonomyVersion {
		t.Errorf("Version = %d, want %d", tax.Version, DefaultTaxonomyVersion)
	}

	want := []string{
		"bug", "style", "naming", "architecture", "performance",
		"security", "test", "docs", "question", "nitpick", "praise", "other",
	}
	if len(tax.Labels) != len(want) {
		t.Fatalf("Labels = %v, want %v", tax.Labels, want)
	}
	for i, label := range want {
		if tax.Labels[i] != label {
			t.Errorf("Labels[%d] = %q, want %q", i, tax.Labels[i], label)
		}
	}

	if !tax.Has(LabelOther) {
		t.Error("DefaultTaxonomy() does not include the required fallback label \"other\"")
	}
}

func TestTaxonomy_Has(t *testing.T) {
	tax := DefaultTaxonomy()

	cases := []struct {
		label string
		want  bool
	}{
		{"bug", true},
		{"other", true},
		{"not-a-real-label", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			if got := tax.Has(tc.label); got != tc.want {
				t.Errorf("Has(%q) = %v, want %v", tc.label, got, tc.want)
			}
		})
	}
}

// TestTaxonomy_UnmarshalJSON_missingOther is the direct check for the
// acceptance criterion "loading a user-supplied taxonomy must reject a
// set that omits other" (TZ.md section 8.5).
func TestTaxonomy_UnmarshalJSON_missingOther(t *testing.T) {
	raw := `{"version": 2, "labels": ["bug", "style", "naming"]}`

	var tax Taxonomy
	err := json.Unmarshal([]byte(raw), &tax)
	if !errors.Is(err, ErrTaxonomyMissingOther) {
		t.Fatalf("Unmarshal() error = %v, want ErrTaxonomyMissingOther", err)
	}
}

func TestTaxonomy_UnmarshalJSON_validSet(t *testing.T) {
	raw := `{"version": 2, "labels": ["bug", "style", "other"]}`

	var tax Taxonomy
	if err := json.Unmarshal([]byte(raw), &tax); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if tax.Version != 2 {
		t.Errorf("Version = %d, want 2", tax.Version)
	}
	if !tax.Has(LabelOther) {
		t.Error("Has(other) = false, want true")
	}
}
