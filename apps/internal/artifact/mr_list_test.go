package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

func sampleMRList() MRList {
	return MRList{
		Header: Header{
			Source: Source{
				GitlabURL: "https://gitlab.example.com",
				FetchedAt: time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		Query: domain.Query{
			GitlabURL: "https://gitlab.example.com",
			UserID:    42,
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  5,
			Group:     "my-group",
			Project:   "my-group/repo",
			Strategy:  domain.StrategyEvents,
			Smoke:     domain.SmokePassed,
		},
		Items: []MRItem{
			{ProjectID: 123, ProjectPath: "my-group/repo", MRIID: 77, CommentCount: 8},
			{ProjectID: 123, ProjectPath: "my-group/repo", MRIID: 91, CommentCount: 6},
		},
	}
}

func TestWriteMRList_jsonRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  MRList
	}{
		{"full query with group, project and point fields", sampleMRList()},
		{"no items", func() MRList {
			d := sampleMRList()
			d.Items = nil
			return d
		}()},
		{"point mode with mr set", func() MRList {
			d := sampleMRList()
			mr := int64(77)
			d.Query.MR = &mr
			d.Query.Group = ""
			d.Query.Project = "my-group/repo"
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mr_list_test.json")
			if err := WriteMRList(tc.doc, FormatJSON, path); err != nil {
				t.Fatalf("WriteMRList() error = %v", err)
			}

			got, err := ReadMRList(path)
			if err != nil {
				t.Fatalf("ReadMRList() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindMRList
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteMRList_yamlRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  MRList
	}{
		{"full query with group, project and point fields", sampleMRList()},
		{"no items", func() MRList {
			d := sampleMRList()
			d.Items = nil
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mr_list_test.yaml")
			if err := WriteMRList(tc.doc, FormatYAML, path); err != nil {
				t.Fatalf("WriteMRList() error = %v", err)
			}

			got, err := ReadMRList(path)
			if err != nil {
				t.Fatalf("ReadMRList() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindMRList
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteMRList_textFormatProducesReadableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mr_list_test.txt")
	if err := WriteMRList(sampleMRList(), FormatText, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	text := string(data)
	if len(text) == 0 {
		t.Fatal("text output is empty")
	}
	if !strings.Contains(text, "mr_list") {
		t.Errorf("text output = %q, want it to mention kind mr_list", text)
	}
	if !strings.Contains(text, "project_path") {
		t.Errorf("text output = %q, want item fields rendered", text)
	}
}

func TestReadMRList_rejectsTextFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mr_list_test.txt")
	if err := WriteMRList(sampleMRList(), FormatText, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	_, err := ReadMRList(path)
	if err == nil {
		t.Fatal("ReadMRList() error = nil, want ErrNotReadable")
	}
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("ReadMRList() error = %v, want ErrNotReadable", err)
	}
}
