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

func sampleCommentList() CommentList {
	return CommentList{
		Header: Header{
			Source: Source{
				GitlabURL: "https://gitlab.example.com",
				FetchedAt: time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		Query: CommentQuery{
			UserID: 42,
			From:   domain.NewDate(2026, time.January, 1),
			To:     domain.NewDate(2026, time.June, 30),
			MRs: []MRRef{
				{ProjectID: 123, MRIID: 77},
			},
			FromArtifact: "artifacts/mr_list_abc123.yaml",
		},
		Items: []CommentItem{
			{
				MRIID: 77,
				Note: domain.Note{
					ID:           456,
					Type:         domain.NoteTypeDiscussion,
					Body:         "looks good to me",
					Author:       domain.Author{ID: 42, Username: "alice"},
					CreatedAt:    time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
					System:       false,
					NoteableID:   999,
					NoteableType: "MergeRequest",
					ProjectID:    123,
				},
			},
			{
				MRIID: 77,
				Note: domain.Note{
					ID:           457,
					Type:         domain.NoteTypeNone,
					Body:         "second comment",
					Author:       domain.Author{ID: 42, Username: "alice"},
					CreatedAt:    time.Date(2026, time.March, 2, 11, 0, 0, 0, time.UTC),
					System:       false,
					NoteableID:   999,
					NoteableType: "MergeRequest",
					ProjectID:    123,
				},
			},
		},
	}
}

func TestWriteCommentList_jsonRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  CommentList
	}{
		{"explicit mrs and from_artifact", sampleCommentList()},
		{"no items", func() CommentList {
			d := sampleCommentList()
			d.Items = nil
			return d
		}()},
		{"no from_artifact, explicit mrs only", func() CommentList {
			d := sampleCommentList()
			d.Query.FromArtifact = ""
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "comment_list_test.json")
			if err := WriteCommentList(tc.doc, FormatJSON, path); err != nil {
				t.Fatalf("WriteCommentList() error = %v", err)
			}

			got, err := ReadCommentList(path)
			if err != nil {
				t.Fatalf("ReadCommentList() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindCommentList
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteCommentList_yamlRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		doc  CommentList
	}{
		{"explicit mrs and from_artifact", sampleCommentList()},
		{"no items", func() CommentList {
			d := sampleCommentList()
			d.Items = nil
			return d
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "comment_list_test.yaml")
			if err := WriteCommentList(tc.doc, FormatYAML, path); err != nil {
				t.Fatalf("WriteCommentList() error = %v", err)
			}

			got, err := ReadCommentList(path)
			if err != nil {
				t.Fatalf("ReadCommentList() error = %v", err)
			}

			want := tc.doc
			want.SchemaVersion = CurrentSchemaVersion
			want.Kind = KindCommentList
			assertEqualJSON(t, got, want)
		})
	}
}

func TestWriteCommentList_textFormatProducesReadableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comment_list_test.txt")
	if err := WriteCommentList(sampleCommentList(), FormatText, path); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	text := string(data)
	if len(text) == 0 {
		t.Fatal("text output is empty")
	}
	if !strings.Contains(text, "comment_list") {
		t.Errorf("text output = %q, want it to mention kind comment_list", text)
	}
	if !strings.Contains(text, "looks good to me") {
		t.Errorf("text output = %q, want comment body rendered", text)
	}
}

func TestReadCommentList_rejectsTextFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "comment_list_test.txt")
	if err := WriteCommentList(sampleCommentList(), FormatText, path); err != nil {
		t.Fatalf("WriteCommentList() error = %v", err)
	}

	_, err := ReadCommentList(path)
	if !errors.Is(err, ErrNotReadable) {
		t.Errorf("ReadCommentList() error = %v, want ErrNotReadable", err)
	}
}
