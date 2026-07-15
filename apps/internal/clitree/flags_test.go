package clitree

import (
	"testing"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		in      string
		want    artifact.Format
		wantErr bool
	}{
		{in: "json", want: artifact.FormatJSON},
		{in: "yaml", want: artifact.FormatYAML},
		{in: "text", want: artifact.FormatText},
		{in: "html", want: artifact.FormatHTML},
		{in: "xml", wantErr: true},
		{in: "", wantErr: true},
		{in: "JSON", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := parseFormat(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseFormat(%q) error = nil, want an error", tt.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFormat(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("parseFormat(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
