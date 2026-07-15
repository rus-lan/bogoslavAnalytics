package artifact

import "fmt"

// FilteredComments is artifact-4: the label-filtered subset of a
// labeled_comments artifact's items (TZ.md section 4.4).
type FilteredComments struct {
	Header
	Query FilteredQuery        `json:"query"`
	Items []LabeledCommentItem `json:"items"`
}

// WriteFilteredComments writes doc to path in the given format. It
// always writes the current schema version and the filtered_comments
// kind, regardless of what doc.SchemaVersion or doc.Kind held on entry.
func WriteFilteredComments(doc FilteredComments, format Format, path string) error {
	doc.SchemaVersion = CurrentSchemaVersion
	doc.Kind = KindFilteredComments

	switch format {
	case FormatText:
		text, err := filteredCommentsText(doc)
		if err != nil {
			return fmt.Errorf("write filtered_comments %q: %w", path, err)
		}
		return writeFile(path, text)
	case FormatHTML:
		page, err := filteredCommentsHTML(doc)
		if err != nil {
			return fmt.Errorf("write filtered_comments %q: %w", path, err)
		}
		return writeFile(path, page)
	default:
		data, err := encode(doc, format)
		if err != nil {
			return fmt.Errorf("write filtered_comments %q: %w", path, err)
		}
		return writeFile(path, data)
	}
}

// ReadFilteredComments reads a filtered_comments artifact from path.
// The format is inferred from the file extension; a write-only path
// (text or html) fails with ErrNotReadable, and a schema_version or
// kind mismatch fails with ErrUnknownSchemaVersion or ErrKindMismatch
// (TZ.md section 4).
func ReadFilteredComments(path string) (FilteredComments, error) {
	var doc FilteredComments
	if err := decodeFile(path, &doc); err != nil {
		return FilteredComments{}, err
	}
	if err := checkHeader(path, doc.Header, KindFilteredComments); err != nil {
		return FilteredComments{}, err
	}
	return doc, nil
}

func filteredCommentsText(doc FilteredComments) ([]byte, error) {
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

// filteredCommentsHTMLTemplate is the shared layout, the
// filtered_comments query section (which surfaces the labels filtered
// on prominently, next to the from_artifact chain it narrows -- TZ.md
// section 4.4), and the label-grouped content section shared with
// labeled_comments.
const filteredCommentsHTMLTemplate = htmlLayout + `
{{define "query"}}
<section class="query">
  <h2>Query</h2>
  <dl>
    <dt>from_artifact</dt><dd>{{.Query.FromArtifact}}</dd>
    <dt class="highlight">labels</dt><dd class="highlight">{{range $i, $l := .Query.Labels}}{{if $i}}, {{end}}{{$l}}{{end}}</dd>
    {{if .Query.From}}<dt>from</dt><dd>{{.Query.From}}</dd>{{end}}
    {{if .Query.To}}<dt>to</dt><dd>{{.Query.To}}</dd>{{end}}
    {{if .Query.Group}}<dt>group</dt><dd>{{.Query.Group}}</dd>{{end}}
    {{if .Query.Project}}<dt>project</dt><dd>{{.Query.Project}}</dd>{{end}}
  </dl>
</section>
{{end}}
` + labeledItemsContentTemplate

func filteredCommentsHTML(doc FilteredComments) ([]byte, error) {
	return renderHTML(filteredCommentsHTMLTemplate, doc)
}
