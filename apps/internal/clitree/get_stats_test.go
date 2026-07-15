package clitree

import (
	"reflect"
	"testing"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/app"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
)

func TestNewGetStatsRequest_mapsFlagsToRequest(t *testing.T) {
	_, flags := buildFlags(t, registerGetStatsFlags, []string{
		"--from-artifact", "artifacts/comment_list_x.yaml",
		"--artifacts-dir", "out",
		"--format", "json",
	})

	got, err := newGetStatsRequest(*flags)
	if err != nil {
		t.Fatalf("newGetStatsRequest() error = %v", err)
	}
	want := app.GetStatsRequest{
		ArtifactPath: "artifacts/comment_list_x.yaml",
		Dir:          "out",
		Format:       artifact.FormatJSON,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("newGetStatsRequest() = %+v, want %+v", got, want)
	}
}

func TestNewGetStatsRequest_defaultsToYAML(t *testing.T) {
	_, flags := buildFlags(t, registerGetStatsFlags, []string{"--from-artifact", "x.yaml"})

	got, err := newGetStatsRequest(*flags)
	if err != nil {
		t.Fatalf("newGetStatsRequest() error = %v", err)
	}
	if got.Format != artifact.FormatYAML {
		t.Errorf("Format = %q, want %q", got.Format, artifact.FormatYAML)
	}
}

func TestNewGetStatsRequest_rejectsUnknownFormat(t *testing.T) {
	_, flags := buildFlags(t, registerGetStatsFlags, []string{"--from-artifact", "x.yaml", "--format", "xml"})
	_, err := newGetStatsRequest(*flags)
	if err == nil {
		t.Fatal("newGetStatsRequest() error = nil, want an error for --format xml")
	}
}
