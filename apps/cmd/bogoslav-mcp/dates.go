package main

import (
	"fmt"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

// parseOptionalDateRange parses an optional from/to pair into the
// *domain.Date pair app.FilterCommentsRequest carries: both empty means
// no extra date filter (nil, nil); app.FilterComments itself rejects
// exactly one being set (app.ErrIncompleteDateFilter), so that check is
// not repeated here. Mirrors bogoslav-cli's identical helper.
func parseOptionalDateRange(from, to string) (*domain.Date, *domain.Date, error) {
	if from == "" && to == "" {
		return nil, nil, nil
	}
	var f, t domain.Date
	var err error
	if from != "" {
		if f, err = domain.ParseDate(from); err != nil {
			return nil, nil, fmt.Errorf("from: %w", err)
		}
	}
	if to != "" {
		if t, err = domain.ParseDate(to); err != nil {
			return nil, nil, fmt.Errorf("to: %w", err)
		}
	}
	return dateOrNil(from, f), dateOrNil(to, t), nil
}

// dateOrNil returns nil when raw is empty, or a pointer to parsed
// otherwise.
func dateOrNil(raw string, parsed domain.Date) *domain.Date {
	if raw == "" {
		return nil
	}
	return &parsed
}
