package app

import (
	"fmt"
	"time"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/classify"
)

// SaveLabelsRequest is the input to SaveLabels, mirroring the
// save_labels tool / classify save command surface (TZ.md section 7.2).
type SaveLabelsRequest struct {
	// CommentListPath is the artifact-2 the labeling was produced for.
	CommentListPath string
	// Taxonomy is the taxonomy the labeling was validated against; nil
	// means classify.DefaultTaxonomy() (TZ.md section 8.5). It must be
	// the exact same taxonomy GetClassifyBatch handed out, or
	// classify.Validate rejects every label as out-of-taxonomy.
	Taxonomy *classify.Taxonomy
	// Labels is the incoming labeling result to validate.
	Labels []classify.NoteLabel

	// Tool, Model and ClassifiedAt build the mandatory classifier
	// provenance block (TZ.md section 8.3).
	Tool         string
	Model        string
	ClassifiedAt time.Time

	Dir    string
	Format artifact.Format
}

// SaveLabelsResult is the output of SaveLabels: the written artifact-3
// and its path.
type SaveLabelsResult struct {
	Doc  artifact.LabeledComments
	Path string
}

// SaveLabels is the shared implementation behind the save_labels MCP
// tool and the classify save CLI command (TZ.md section 7.2): validate
// an incoming labeling against artifact-2's batch and the taxonomy via
// classify.Validate, and only on success build the classifier provenance
// block and write artifact-3. A labeling that fails validation returns
// the *classify.ValidationError and writes nothing:
// artifact.WriteLabeledComments is never reached on that path, so no
// file is ever created for a rejected labeling (TZ.md section 8.1).
//
// Source (gitlab_url, fetched_at) is carried over unchanged from
// artifact-2's own header: SaveLabels makes no GitLab API calls, so
// nothing new was fetched.
func SaveLabels(req SaveLabelsRequest) (SaveLabelsResult, error) {
	doc2, err := artifact.ReadCommentList(req.CommentListPath)
	if err != nil {
		return SaveLabelsResult{}, fmt.Errorf("save labels: read %q: %w", req.CommentListPath, err)
	}

	notes := notesOf(doc2.Items)
	taxonomy := resolveTaxonomy(req.Taxonomy)

	labeled, err := classify.Validate(taxonomy, notes, req.Labels)
	if err != nil {
		return SaveLabelsResult{}, fmt.Errorf("save labels: %w", err)
	}

	items := make([]artifact.LabeledCommentItem, len(doc2.Items))
	for i, it := range doc2.Items {
		items[i] = artifact.LabeledCommentItem{MRIID: it.MRIID, LabeledNote: labeled[i]}
	}

	classifier := classify.NewClassifier(req.Tool, req.Model, taxonomy.Version, req.ClassifiedAt)

	hash, err := labelArtifactHash(notes, req.Model, taxonomy.Version)
	if err != nil {
		return SaveLabelsResult{}, fmt.Errorf("save labels: %w", err)
	}

	dir := outDir(req.Dir)
	format := outFormat(req.Format)
	path, err := artifactPath(dir, artifact.KindLabeledComments, hash, format)
	if err != nil {
		return SaveLabelsResult{}, fmt.Errorf("save labels: %w", err)
	}

	// WriteLabeledComments only fixes up SchemaVersion/Kind on its own
	// copy of doc, not on this local variable, so Result.Doc is set
	// explicitly here to keep it identical to what lands on disk.
	header := doc2.Header
	header.SchemaVersion = artifact.CurrentSchemaVersion
	header.Kind = artifact.KindLabeledComments

	doc := artifact.LabeledComments{
		Header:     header,
		Query:      doc2.Query,
		Taxonomy:   artifact.Taxonomy{Version: taxonomy.Version, Labels: taxonomy.Labels},
		Classifier: classifier,
		Items:      items,
	}
	if err := artifact.WriteLabeledComments(doc, format, path); err != nil {
		return SaveLabelsResult{}, fmt.Errorf("save labels: %w", err)
	}

	return SaveLabelsResult{Doc: doc, Path: path}, nil
}
