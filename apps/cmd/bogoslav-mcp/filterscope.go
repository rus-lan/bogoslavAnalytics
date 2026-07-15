package main

import (
	"context"
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/gitlab"
)

// resolveFilterScope resolves group/project into the numeric ids
// app.FilterCommentsRequest actually filters by (TZ.md section 7.2, and
// FilterCommentsRequest's own doc comment: a caller resolves this ahead
// of time if it wants group/project filtering to actually narrow
// anything). It calls GitLab only for whichever of the two is non-empty
// -- the same glue bogoslav-cli's filter-comments command already runs,
// duplicated here rather than shared, since bogoslav-cli is its own
// package main and cannot be imported.
func resolveFilterScope(ctx context.Context, client *gitlab.Client, group, project string) (projectIDs []int64, projectID *int64, err error) {
	if project != "" {
		id, err := resolveProjectID(ctx, client, project)
		if err != nil {
			return nil, nil, err
		}
		projectID = &id
	}
	if group != "" {
		projects, err := client.GroupProjects(ctx, buildGitlabID(group))
		if err != nil {
			return nil, nil, fmt.Errorf("resolve group %q: %w", group, err)
		}
		projectIDs = make([]int64, len(projects))
		for i, p := range projects {
			projectIDs[i] = p.ID
		}
	}
	return projectIDs, projectID, nil
}

// resolveProjectID resolves a project value to its numeric id: parsed
// directly if it is already all digits, or via one GetProject call if it
// is a namespaced path.
func resolveProjectID(ctx context.Context, client *gitlab.Client, project string) (int64, error) {
	if n, ok := parseNumericID(project); ok {
		return n, nil
	}
	p, err := client.GetProject(ctx, gitlab.PathID(project))
	if err != nil {
		return 0, fmt.Errorf("resolve project %q: %w", project, err)
	}
	return p.ID, nil
}
