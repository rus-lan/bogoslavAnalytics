package artifact

import "fmt"

// CommentList is artifact-2: the comments a user left across a set of
// merge requests (TZ.md section 4.2).
type CommentList struct {
	Header
	Query CommentQuery  `json:"query"`
	Items []CommentItem `json:"items"`
}

// WriteCommentList writes doc to path in the given format. It always
// writes the current schema version and the comment_list kind,
// regardless of what doc.SchemaVersion or doc.Kind held on entry.
func WriteCommentList(doc CommentList, format Format, path string) error {
	doc.SchemaVersion = CurrentSchemaVersion
	doc.Kind = KindCommentList

	switch format {
	case FormatText:
		text, err := commentListText(doc)
		if err != nil {
			return fmt.Errorf("write comment_list %q: %w", path, err)
		}
		return writeFile(path, text)
	case FormatHTML:
		page, err := commentListHTML(doc)
		if err != nil {
			return fmt.Errorf("write comment_list %q: %w", path, err)
		}
		return writeFile(path, page)
	default:
		data, err := encode(doc, format)
		if err != nil {
			return fmt.Errorf("write comment_list %q: %w", path, err)
		}
		return writeFile(path, data)
	}
}

// ReadCommentList reads a comment_list artifact from path, for example
// as the from_artifact input to a labeled_comments or filtered_comments
// step. The format is inferred from the file extension; a write-only
// path (text or html) fails with ErrNotReadable, and a schema_version
// or kind mismatch fails with ErrUnknownSchemaVersion or
// ErrKindMismatch (TZ.md section 4).
func ReadCommentList(path string) (CommentList, error) {
	var doc CommentList
	if err := decodeFile(path, &doc); err != nil {
		return CommentList{}, err
	}
	if err := checkHeader(path, doc.Header, KindCommentList); err != nil {
		return CommentList{}, err
	}
	return doc, nil
}

func commentListText(doc CommentList) ([]byte, error) {
	header, err := renderTextHeader(doc.Header, doc.Query)
	if err != nil {
		return nil, err
	}
	items, err := renderTextSection("items", doc.Items)
	if err != nil {
		return nil, err
	}
	return []byte(header + "\n" + items), nil
}

// commentListHTMLTemplate is the shared layout plus the comment_list
// query and content sections. Content groups comments by the merge
// request they belong to (TZ.md section 4.2). Comment bodies pass
// through {{.Body}} inside an html/template action, so GitLab user
// content is contextually auto-escaped rather than injected raw.
const commentListHTMLTemplate = htmlLayout + `
{{define "query"}}
<section class="query">
  <h2>Query</h2>
  <dl>
    <dt>user_id</dt><dd>{{.Query.UserID}}</dd>
    <dt>from</dt><dd>{{.Query.From}}</dd>
    <dt>to</dt><dd>{{.Query.To}}</dd>
    {{if .Query.FromArtifact}}<dt>from_artifact</dt><dd>{{.Query.FromArtifact}}</dd>{{end}}
    <dt>merge requests</dt><dd>{{len .Query.MRs}}</dd>
  </dl>
</section>
{{end}}

{{define "content"}}
<section class="content">
{{$groups := groupCommentsByMR .Items}}
  <h2>Comments <span class="count">({{len .Items}} across {{len $groups}} merge request{{if ne (len $groups) 1}}s{{end}})</span></h2>
  {{range $groups}}
    <h3>Project {{.ProjectID}}, MR !{{.MRIID}} <span class="count">({{len .Items}})</span></h3>
    {{range .Items}}
      <article class="comment">
        <header><span class="author">{{.Author.Username}}</span><time>{{fmtTime .CreatedAt}}</time></header>
        <div class="body">{{.Body}}</div>
      </article>
    {{end}}
  {{end}}
</section>
{{end}}
`

func commentListHTML(doc CommentList) ([]byte, error) {
	return renderHTML(commentListHTMLTemplate, doc)
}
