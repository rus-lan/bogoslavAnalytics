package cache

import "testing"

func TestFileName_buildsKindHashExt(t *testing.T) {
	cases := []struct {
		name string
		kind string
		hash string
		ext  string
		want string
	}{
		{
			name: "mr_list yaml",
			kind: "mr_list",
			hash: "abc123",
			ext:  ExtYAML,
			want: "mr_list_abc123.yaml",
		},
		{
			name: "comment_list json",
			kind: "comment_list",
			hash: "def456",
			ext:  ExtJSON,
			want: "comment_list_def456.json",
		},
		{
			name: "labeled_comments text is write-only but still a valid name",
			kind: "labeled_comments",
			hash: "ghi789",
			ext:  ExtText,
			want: "labeled_comments_ghi789.txt",
		},
		{
			name: "filtered_comments",
			kind: "filtered_comments",
			hash: "jkl012",
			ext:  ExtYAML,
			want: "filtered_comments_jkl012.yaml",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FileName(tc.kind, tc.hash, tc.ext); got != tc.want {
				t.Errorf("FileName(%q, %q, %q) = %q, want %q", tc.kind, tc.hash, tc.ext, got, tc.want)
			}
		})
	}
}
