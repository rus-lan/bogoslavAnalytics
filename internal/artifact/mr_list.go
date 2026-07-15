package artifact

import (
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// MRList is artifact-1: the merge requests found by find_mrs, each with
// its exact comment count for the query's user and date range (TZ.md
// section 4.1).
type MRList struct {
	Header
	Query domain.Query `json:"query"`
	Items []MRItem     `json:"items"`
}

// WriteMRList writes doc to path in the given format. It always writes
// the current schema version and the mr_list kind, regardless of what
// doc.SchemaVersion or doc.Kind held on entry.
func WriteMRList(doc MRList, format Format, path string) error {
	doc.SchemaVersion = CurrentSchemaVersion
	doc.Kind = KindMRList

	switch format {
	case FormatText:
		text, err := mrListText(doc)
		if err != nil {
			return fmt.Errorf("write mr_list %q: %w", path, err)
		}
		return writeFile(path, text)
	case FormatHTML:
		page, err := mrListHTML(doc)
		if err != nil {
			return fmt.Errorf("write mr_list %q: %w", path, err)
		}
		return writeFile(path, page)
	default:
		data, err := encode(doc, format)
		if err != nil {
			return fmt.Errorf("write mr_list %q: %w", path, err)
		}
		return writeFile(path, data)
	}
}

// ReadMRList reads an mr_list artifact from path. The format is
// inferred from the file extension; a write-only path (text or html)
// fails with ErrNotReadable, and a schema_version or kind mismatch
// fails with ErrUnknownSchemaVersion or ErrKindMismatch (TZ.md section
// 4).
func ReadMRList(path string) (MRList, error) {
	var doc MRList
	if err := decodeFile(path, &doc); err != nil {
		return MRList{}, err
	}
	if err := checkHeader(path, doc.Header, KindMRList); err != nil {
		return MRList{}, err
	}
	return doc, nil
}

func mrListText(doc MRList) ([]byte, error) {
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

// mrListHTMLTemplate is the shared layout plus the mr_list query and
// content sections. The query section surfaces more_than, strategy and
// smoke prominently: a reader needs to know at a glance whether these
// counts came from the events or bruteforce strategy, since that
// changes what the numbers mean (TZ.md section 5).
const mrListHTMLTemplate = htmlLayout + `
{{define "query"}}
<section class="query">
  <h2>Query</h2>
  <dl>
    <dt>user_id</dt><dd>{{.Query.UserID}}</dd>
    <dt>from</dt><dd>{{.Query.From}}</dd>
    <dt>to</dt><dd>{{.Query.To}}</dd>
    {{if .Query.Group}}<dt>group</dt><dd>{{.Query.Group}}</dd>{{end}}
    {{if .Query.Project}}<dt>project</dt><dd>{{.Query.Project}}</dd>{{end}}
    {{if .Query.MR}}<dt>mr</dt><dd>{{.Query.MR}}</dd>{{end}}
    <dt class="highlight">more_than</dt><dd class="highlight">{{.Query.MoreThan}}</dd>
    <dt class="highlight">strategy</dt><dd class="highlight">{{.Query.Strategy}}</dd>
    <dt class="highlight">smoke</dt><dd class="highlight">{{.Query.Smoke}}</dd>
  </dl>
</section>
{{end}}

{{define "content"}}
<section class="content">
  <h2>Merge requests <span class="count">({{len .Items}})</span></h2>
  <div class="table-scroll">
  <table>
    <thead><tr><th>Merge request</th><th>Project</th><th>Comments by user</th><th>Created</th><th>Updated</th></tr></thead>
    <tbody>
    {{range .Items}}
      <tr>
        <td>{{if .WebURL}}<a href="{{.WebURL}}">{{if .Title}}{{.Title}}{{else}}!{{.MRIID}}{{end}}</a>{{else}}{{if .Title}}{{.Title}}{{else}}!{{.MRIID}}{{end}}{{end}}</td>
        <td>{{if .ProjectPath}}{{.ProjectPath}}{{else}}{{.ProjectID}}{{end}}</td>
        <td>{{.CommentCount}}</td>
        <td>{{fmtTime .CreatedAt}}</td>
        <td>{{fmtTime .UpdatedAt}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>
  </div>
</section>
{{end}}
`

func mrListHTML(doc MRList) ([]byte, error) {
	return renderHTML(mrListHTMLTemplate, doc)
}
