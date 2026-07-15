package filter

import (
	"github.com/rus-lan/bogoslavAnalytics/internal/artifact"
	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

// CommentsByDate keeps only comment_list rows whose CreatedAt falls inside
// the inclusive UTC instant range r expands to (TZ.md section 5.4).
func CommentsByDate(items []artifact.CommentItem, r domain.DateRange) []artifact.CommentItem {
	out := make([]artifact.CommentItem, 0, len(items))
	for _, it := range items {
		if r.Contains(it.CreatedAt) {
			out = append(out, it)
		}
	}
	return out
}
