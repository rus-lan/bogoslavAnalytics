package app

import (
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/cache"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
	"github.com/rus-lan/bogoslavAnalytics/apps/internal/filter"
)

// FilterCommentsRequest is the input to FilterComments, mirroring the
// filter_comments tool / filter command surface (TZ.md section 7.2).
//
// FilterComments makes no GitLab API calls: filter/ already assumes a
// group's member project ids are resolved by its caller (see
// filter.MRsByGroup's own doc comment), and FilterComments extends that
// same convention one level up. Group and Project only appear here for
// provenance (recorded on the output query, TZ.md section 4.5); the
// numeric ids that actually drive filtering are ProjectIDs/ProjectID,
// which a caller resolves ahead of time (for example via
// FindMRsClient.GroupProjects/GetProject) if it wants --group/--project
// filtering to actually narrow anything.
type FilterCommentsRequest struct {
	LabeledCommentsPath string
	Labels              []string

	From *domain.Date
	To   *domain.Date

	Group      string
	Project    string
	ProjectIDs []int64
	ProjectID  *int64

	Dir    string
	Format artifact.Format
}

// FilterCommentsResult is the output of FilterComments: the written
// artifact-4 and its path.
type FilterCommentsResult struct {
	Doc  artifact.FilteredComments
	Path string
}

// FilterComments is the shared implementation behind the
// filter_comments MCP tool and the filter CLI command (TZ.md section
// 7.2): read artifact-3, narrow it by label and the optional date/
// group/project filters via filter/, and write artifact-4.
//
// Source (gitlab_url, fetched_at) is carried over unchanged from
// artifact-3's own header: FilterComments makes no GitLab API calls, so
// nothing new was fetched.
func FilterComments(req FilterCommentsRequest) (FilterCommentsResult, error) {
	if len(req.Labels) == 0 {
		return FilterCommentsResult{}, ErrNoLabels
	}
	if (req.From == nil) != (req.To == nil) {
		return FilterCommentsResult{}, ErrIncompleteDateFilter
	}

	doc3, err := artifact.ReadLabeledComments(req.LabeledCommentsPath)
	if err != nil {
		return FilterCommentsResult{}, fmt.Errorf("filter comments: read %q: %w", req.LabeledCommentsPath, err)
	}

	items := filter.ByLabel(doc3.Items, req.Labels...)
	if req.From != nil && req.To != nil {
		r, err := domain.NewDateRange(*req.From, *req.To)
		if err != nil {
			return FilterCommentsResult{}, fmt.Errorf("filter comments: %w", err)
		}
		items = filter.ByDate(items, r)
	}
	if req.ProjectID != nil {
		items = filter.ByProject(items, *req.ProjectID)
	}
	// Guard on req.Group (whether a group was requested at all), not on
	// len(req.ProjectIDs): a group that resolves to zero projects -- or
	// one whose projects the caller's token cannot see -- has ProjectIDs
	// == [], and a length guard would then skip ByGroup entirely, keeping
	// every item instead of none. req.Group is set unconditionally by
	// both the CLI and MCP callers whenever --group/group was named, so
	// it is the actual "was a group requested" signal.
	if req.Group != "" {
		items = filter.ByGroup(items, req.ProjectIDs)
	}

	query := artifact.FilteredQuery{
		FromArtifact: req.LabeledCommentsPath,
		Labels:       req.Labels,
		From:         req.From,
		To:           req.To,
		Group:        req.Group,
		Project:      req.Project,
	}

	hash, err := cache.Hash(query)
	if err != nil {
		return FilterCommentsResult{}, fmt.Errorf("filter comments: %w", err)
	}
	dir := outDir(req.Dir)
	format := outFormat(req.Format)
	path, err := artifactPath(dir, artifact.KindFilteredComments, hash, format)
	if err != nil {
		return FilterCommentsResult{}, fmt.Errorf("filter comments: %w", err)
	}

	// WriteFilteredComments only fixes up SchemaVersion/Kind on its own
	// copy of doc, not on this local variable, so Result.Doc is set
	// explicitly here to keep it identical to what lands on disk.
	header := doc3.Header
	header.SchemaVersion = artifact.CurrentSchemaVersion
	header.Kind = artifact.KindFilteredComments

	doc := artifact.FilteredComments{
		Header: header,
		Query:  query,
		Items:  items,
	}
	if err := artifact.WriteFilteredComments(doc, format, path); err != nil {
		return FilterCommentsResult{}, fmt.Errorf("filter comments: %w", err)
	}

	return FilterCommentsResult{Doc: doc, Path: path}, nil
}
