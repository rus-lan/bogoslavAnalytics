package gitlab

import (
	"net/url"
	"strconv"
)

// ID identifies a GitLab group or project the way the REST API's :id path
// parameter accepts it. On GitLab 18.11 every :id row across the API
// reference reads "The ID or URL-encoded path of the project" (or group):
// both a numeric id and a namespaced path (e.g. "my-group/my-project") are
// valid. The REST docs spell out the encoding rule: "make sure that the
// NAMESPACE/PROJECT_PATH is URL-encoded. For example, / is represented by
// %2F", with the documented example GET /api/v4/projects/diaspora%2Fdiaspora.
//
// Documented caveat, worth repeating here since it explains a confusing
// failure mode: "If there's something in front of the API (for example,
// Apache), ensure that it doesn't decode the URL-encoded path parameters."
// A proxy that decodes %2F before GitLab sees it turns a valid path
// lookup into a 404.
type ID struct {
	path     string
	numeric  int64
	fromPath bool
}

// NumericID builds an ID from a numeric project or group id.
func NumericID(id int64) ID {
	return ID{numeric: id}
}

// PathID builds an ID from a namespaced path, such as "my-group/my-project"
// or a nested "group/subgroup/project". Every "/" in path is percent-
// encoded as "%2F" on the wire; see (ID).segment.
func PathID(path string) ID {
	return ID{path: path, fromPath: true}
}

// segment renders id as the literal request-path segment GitLab expects: a
// plain decimal integer for a numeric id, or the path with url.PathEscape
// applied (which turns every "/" into "%2F", along with any other
// character a single path segment cannot carry unescaped) for a path id.
//
// The result must be placed directly into the path string handed to
// (*Client).newRequest, and that path string must go through url.Parse as
// a whole (which is what building the full URL string and handing it to
// http.NewRequestWithContext already does) -- never by assigning to
// req.URL.Path after the fact. net/url's URL keeps a decoded Path and a
// separately encoded RawPath; setting Path alone loses the encoded form,
// and "%2F" would be read back out as a plain "/" the moment something
// re-derives EscapedPath from Path instead of RawPath.
func (id ID) segment() string {
	if id.fromPath {
		return url.PathEscape(id.path)
	}
	return strconv.FormatInt(id.numeric, 10)
}

// String renders id for logging and error messages: the decimal digits for
// a numeric id, or the plain (unescaped) path for a path id.
func (id ID) String() string {
	if id.fromPath {
		return id.path
	}
	return strconv.FormatInt(id.numeric, 10)
}
