package app

import (
	"strconv"

	"github.com/rus-lan/bogoslav-analytics/apps/internal/artifact"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/domain"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/gitlab"
	"github.com/rus-lan/bogoslav-analytics/apps/internal/search"
)

// buildID converts a --group/--project value into a gitlab.ID the same
// way ResolveUser resolves --user (TZ.md section 5.0): a value made only
// of digits becomes a gitlab.NumericID, anything else becomes a
// gitlab.PathID, since GitLab 18.11 accepts a path directly in every
// :id path parameter (TZ.md section 14, item 1).
func buildID(value string) gitlab.ID {
	if n, ok := parseNumericID(value); ok {
		return gitlab.NumericID(n)
	}
	return gitlab.PathID(value)
}

// parseNumericID reports whether value is made only of decimal digits,
// returning its numeric value when it is.
func parseNumericID(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// toSearchParams converts a normalized domain.Query into the
// search.Params search.Find accepts (TZ.md sections 5.7 and 14, item 9):
// the query's Group/Project path strings become a gitlab.ID via buildID,
// the same all-digits-is-numeric rule ResolveUser applies to --user.
// Project wins over Group when both are set, matching search.Scope's own
// documented precedence; when neither is set, the scope stays fully
// unscoped (every project visible to the token).
func toSearchParams(q domain.Query) search.Params {
	return search.Params{
		UserID:   q.UserID,
		Range:    domain.DateRange{From: q.From, To: q.To},
		MoreThan: q.MoreThan,
		Scope:    scopeFor(q.Group, q.Project),
	}
}

// scopeFor builds a search.Scope from --group/--project path strings.
func scopeFor(group, project string) search.Scope {
	switch {
	case project != "":
		id := buildID(project)
		return search.Scope{ProjectID: &id}
	case group != "":
		id := buildID(group)
		return search.Scope{GroupID: &id}
	default:
		return search.Scope{}
	}
}

// toMRList converts a search.Result into artifact-1 (TZ.md section 4.2):
// every domain.MergeRequest field the html/json/yaml views can show --
// ProjectPath, Title, WebURL, CreatedAt, UpdatedAt, alongside ProjectID,
// MRIID and CommentCount -- is carried across into the matching
// artifact.MRItem field. This is the converter TZ.md section 14, item 10
// left open: artifact.MRItem.ProjectPath had no producer until this
// function existed.
func toMRList(header artifact.Header, query domain.Query, result search.Result) artifact.MRList {
	query.Strategy = result.Strategy
	query.Smoke = result.Smoke

	items := make([]artifact.MRItem, len(result.Items))
	for i, mr := range result.Items {
		items[i] = artifact.MRItem{
			ProjectID:    mr.ProjectID,
			ProjectPath:  mr.ProjectPath,
			MRIID:        mr.IID,
			CommentCount: mr.CommentCount,
			Title:        mr.Title,
			WebURL:       mr.WebURL,
			CreatedAt:    mr.CreatedAt,
			UpdatedAt:    mr.UpdatedAt,
		}
	}
	return artifact.MRList{Header: header, Query: query, Items: items}
}
