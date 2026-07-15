package artifact

// commentMRGroup is the comment_list html view's presentation grouping:
// every CommentItem that shares an (ProjectID, MRIID) pair, in the
// order that pair first appears.
type commentMRGroup struct {
	ProjectID int64
	MRIID     int64
	Items     []CommentItem
}

// groupCommentsByMR groups comment_list items by merge request, in
// first-seen order, for display (TZ.md section 4.2: "comments grouped
// by MR"). This is a presentation-only regrouping of already-selected
// items, not a filter: every input item appears in exactly one group.
func groupCommentsByMR(items []CommentItem) []commentMRGroup {
	var groups []commentMRGroup
	index := make(map[[2]int64]int)

	for _, item := range items {
		key := [2]int64{item.ProjectID, item.MRIID}
		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, commentMRGroup{ProjectID: item.ProjectID, MRIID: item.MRIID})
		}
		groups[i].Items = append(groups[i].Items, item)
	}
	return groups
}

// labelGroup is the labeled_comments / filtered_comments html view's
// presentation grouping: every LabeledCommentItem that shares a Label,
// in the order that label first appears.
type labelGroup struct {
	Label string
	Items []LabeledCommentItem
}

// groupLabeledByLabel groups labeled_comments/filtered_comments items
// by their semantic label, in first-seen order, for display (the
// "grouped BY LABEL" requirement). This is a presentation-only
// regrouping of already-selected items, not a filter.
func groupLabeledByLabel(items []LabeledCommentItem) []labelGroup {
	var groups []labelGroup
	index := make(map[string]int)

	for _, item := range items {
		i, ok := index[item.Label]
		if !ok {
			i = len(groups)
			index[item.Label] = i
			groups = append(groups, labelGroup{Label: item.Label})
		}
		groups[i].Items = append(groups[i].Items, item)
	}
	return groups
}
