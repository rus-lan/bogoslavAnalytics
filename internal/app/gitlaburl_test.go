package app

import "testing"

func TestSanitizeGitlabURL_stripsUserinfo(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "user and password",
			in:   "http://gluser:URLCANARY-99999@127.0.0.1:44297",
			want: "http://127.0.0.1:44297",
		},
		{
			name: "oauth2 token idiom",
			in:   "https://oauth2:glpat-CANARY@gitlab.example.com",
			want: "https://gitlab.example.com",
		},
		{
			name: "username only, no password",
			in:   "https://gluser@gitlab.example.com",
			want: "https://gitlab.example.com",
		},
		{
			name: "no userinfo at all",
			in:   "https://gitlab.example.com",
			want: "https://gitlab.example.com",
		},
		{
			name: "path and query survive",
			in:   "https://gluser:secret@gitlab.example.com/api/v4?x=1",
			want: "https://gitlab.example.com/api/v4?x=1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeGitlabURL(tc.in)
			if got != tc.want {
				t.Errorf("sanitizeGitlabURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeGitlabURL_unparsableURLReturnedUnchanged(t *testing.T) {
	// A control character makes url.Parse fail; sanitizeGitlabURL must
	// not panic or turn this into an empty string, it just passes the
	// value through untouched.
	in := "http://gitlab.example.com/\x7f"
	if got := sanitizeGitlabURL(in); got != in {
		t.Errorf("sanitizeGitlabURL(%q) = %q, want unchanged input", in, got)
	}
}
