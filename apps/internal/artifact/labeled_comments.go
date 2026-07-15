package artifact

import (
	"fmt"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
)

// LabeledComments is artifact-3: a comment_list plus the semantic label
// assigned to each comment by the calling agent, and the classifier
// provenance block recording how the labeling was done (TZ.md section
// 4.3 and section 8.3).
type LabeledComments struct {
	Header
	Query      CommentQuery         `json:"query"`
	Taxonomy   Taxonomy             `json:"taxonomy"`
	Classifier domain.Classifier    `json:"classifier"`
	Items      []LabeledCommentItem `json:"items"`
}

// WriteLabeledComments writes doc to path in the given format. It
// always writes the current schema version and the labeled_comments
// kind, regardless of what doc.SchemaVersion or doc.Kind held on entry.
//
// The classifier provenance block is mandatory (TZ.md section 8.3): if
// any of its four fields is unset, WriteLabeledComments fails with
// ErrMissingClassifier and writes nothing.
func WriteLabeledComments(doc LabeledComments, format Format, path string) error {
	if err := validateClassifier(doc.Classifier); err != nil {
		return fmt.Errorf("write labeled_comments %q: %w", path, err)
	}

	doc.SchemaVersion = CurrentSchemaVersion
	doc.Kind = KindLabeledComments

	switch format {
	case FormatText:
		text, err := labeledCommentsText(doc)
		if err != nil {
			return fmt.Errorf("write labeled_comments %q: %w", path, err)
		}
		return writeFile(path, text)
	case FormatHTML:
		page, err := labeledCommentsHTML(doc)
		if err != nil {
			return fmt.Errorf("write labeled_comments %q: %w", path, err)
		}
		return writeFile(path, page)
	default:
		data, err := encode(doc, format)
		if err != nil {
			return fmt.Errorf("write labeled_comments %q: %w", path, err)
		}
		return writeFile(path, data)
	}
}

// ReadLabeledComments reads a labeled_comments artifact from path, for
// example as the from_artifact input to a filtered_comments step. The
// format is inferred from the file extension; a write-only path (text
// or html) fails with ErrNotReadable, and a schema_version or kind
// mismatch fails with ErrUnknownSchemaVersion or ErrKindMismatch
// (TZ.md section 4).
func ReadLabeledComments(path string) (LabeledComments, error) {
	var doc LabeledComments
	if err := decodeFile(path, &doc); err != nil {
		return LabeledComments{}, err
	}
	if err := checkHeader(path, doc.Header, KindLabeledComments); err != nil {
		return LabeledComments{}, err
	}
	return doc, nil
}

// validateClassifier checks that every field of the classifier
// provenance block is populated (TZ.md section 8.3).
func validateClassifier(c domain.Classifier) error {
	if c.Tool == "" || c.Model == "" || c.TaxonomyVersion <= 0 || c.ClassifiedAt.IsZero() {
		return ErrMissingClassifier
	}
	return nil
}

func labeledCommentsText(doc LabeledComments) ([]byte, error) {
	header, err := renderTextHeader(doc.Header, doc.Query)
	if err != nil {
		return nil, err
	}
	taxonomy, err := renderTextSection("taxonomy", doc.Taxonomy)
	if err != nil {
		return nil, err
	}
	classifier, err := renderTextSection("classifier", doc.Classifier)
	if err != nil {
		return nil, err
	}
	items, err := renderTextSection("items", doc.Items)
	if err != nil {
		return nil, err
	}
	return []byte(header + "\n" + taxonomy + "\n" + classifier + "\n" + items), nil
}

// labeledCommentsHTMLTemplate is the shared layout, the labeled_comments
// query section (which also surfaces the classifier provenance block
// prominently, right next to the query rather than buried among the
// items — TZ.md section 8.3), and the label-grouped content section
// shared with filtered_comments.
const labeledCommentsHTMLTemplate = htmlLayout + `
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
<section class="provenance">
  <h2>Classifier provenance</h2>
  <dl>
    <dt class="highlight">tool</dt><dd class="highlight">{{.Classifier.Tool}}</dd>
    <dt class="highlight">model</dt><dd class="highlight">{{.Classifier.Model}}</dd>
    <dt class="highlight">taxonomy_version</dt><dd class="highlight">{{.Classifier.TaxonomyVersion}}</dd>
    <dt class="highlight">classified_at</dt><dd class="highlight">{{fmtTime .Classifier.ClassifiedAt}}</dd>
  </dl>
  <p>Taxonomy v{{.Taxonomy.Version}}: {{range $i, $l := .Taxonomy.Labels}}{{if $i}}, {{end}}{{$l}}{{end}}</p>
</section>
{{end}}
` + labeledItemsContentTemplate

func labeledCommentsHTML(doc LabeledComments) ([]byte, error) {
	return renderHTML(labeledCommentsHTMLTemplate, doc)
}
