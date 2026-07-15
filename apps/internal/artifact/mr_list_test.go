package artifact

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
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

// TestWriteMRList_htmlFallsBackToProjectIDWhenPathEmpty checks that the
// Project column still identifies the row when ProjectPath is empty
// (its normal state today, since nothing produces it yet): it must
// show project_id instead of rendering an empty cell.
func TestWriteMRList_htmlFallsBackToProjectIDWhenPathEmpty(t *testing.T) {
	doc := sampleMRList()
	doc.Items = []MRItem{{ProjectID: 999, MRIID: 77, CommentCount: 8}}

	path := filepath.Join(t.TempDir(), "mr_list_no_path_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if !strings.Contains(html, "<td>999</td>") {
		t.Errorf("html output does not fall back to project_id in the Project cell:\n%s", html)
	}

	// The Project column is the <td> right after the merge request
	// column; it must carry the fallback id, not render blank, even
	// though CreatedAt/UpdatedAt (the last two columns) legitimately
	// render blank when zero-valued.
	const mrCell = "<td>!77</td>"
	afterMRCell := strings.Index(html, mrCell)
	if afterMRCell < 0 {
		t.Fatalf("could not locate the merge request cell in html output:\n%s", html)
	}
	rest := html[afterMRCell+len(mrCell):]
	start := strings.Index(rest, "<td>")
	end := strings.Index(rest, "</td>")
	if start < 0 || end < 0 || start > end {
		t.Fatalf("could not locate the Project cell in html output:\n%s", html)
	}
	projectCell := rest[start+len("<td>") : end]
	if projectCell != "999" {
		t.Errorf("Project cell = %q, want %q", projectCell, "999")
	}
}

// TestWriteMRList_htmlShowsProjectPathWhenSet is the regression check
// for the fallback above: an item that does carry a ProjectPath must
// still show the path, not the numeric id.
func TestWriteMRList_htmlShowsProjectPathWhenSet(t *testing.T) {
	doc := sampleMRList()

	path := filepath.Join(t.TempDir(), "mr_list_with_path_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if !strings.Contains(html, "<td>my-group/repo</td>") {
		t.Errorf("html output does not show the set project_path:\n%s", html)
	}
}

// TestWriteMRList_htmlLinkTextFallsBackToIID checks the merge request
// column: with no Title, the link text must be "!<iid>" with no
// leading empty project-path fragment; with a Title set, it must show
// the title instead.
func TestWriteMRList_htmlLinkTextFallsBackToIID(t *testing.T) {
	doc := sampleMRList()
	doc.Items = []MRItem{{ProjectID: 999, MRIID: 77, CommentCount: 8}}

	path := filepath.Join(t.TempDir(), "mr_list_no_title_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html := string(data)

	if !strings.Contains(html, "<td>!77</td>") {
		t.Errorf("html output does not fall back to !<iid> for the link text:\n%s", html)
	}

	doc.Items[0].Title = "fix the thing"
	path = filepath.Join(t.TempDir(), "mr_list_with_title_test.html")
	if err := WriteMRList(doc, FormatHTML, path); err != nil {
		t.Fatalf("WriteMRList() error = %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	html = string(data)

	if !strings.Contains(html, "fix the thing") {
		t.Errorf("html output does not show the set title as link text:\n%s", html)
	}
	if strings.Contains(html, "!77") {
		t.Errorf("html output falls back to !<iid> even though Title is set:\n%s", html)
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
