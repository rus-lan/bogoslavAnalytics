package gitlab

import "context"

const (
	// perPage is always used at the maximum GitLab allows (TZ.md section
	// 6.6: "per_page максимум 100 -- всегда использовать 100").
	perPage = 100

	// maxPage bounds how many full pages paginate will walk before giving
	// up and reporting ErrPageLimitReached: perPage(100) * maxPage(100) =
	// 10,000 records, the point past which GitLab stops sending
	// x-total/x-total-pages headers (TZ.md section 6.7). Only offset
	// pagination exists for these endpoints -- there is no keyset
	// alternative to fall back on (TZ.md section 5.6.8).
	maxPage = 100
)

// fetchPage retrieves one 1-based page of results.
type fetchPage[T any] func(ctx context.Context, page int) ([]T, error)

// paginate walks every page via offset pagination (page + per_page=100),
// stopping at the first short page (fewer than perPage items), which is
// how the end of an offset-paginated list is detected. If maxPage full
// pages are consumed without a short page, it stops -- deeper paging is
// never attempted -- and returns every item collected so far together
// with ErrPageLimitReached, so the caller can split the request into date
// sub-windows instead (TZ.md section 6.7).
func paginate[T any](ctx context.Context, fetch fetchPage[T]) ([]T, error) {
	var all []T
	for page := 1; page <= maxPage; page++ {
		items, err := fetch(ctx, page)
		if err != nil {
			return all, err
		}
		all = append(all, items...)
		if len(items) < perPage {
			return all, nil
		}
		if page == maxPage {
			return all, ErrPageLimitReached
		}
	}
	return all, nil
}
