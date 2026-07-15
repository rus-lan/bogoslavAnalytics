package domain

import "errors"

// Sentinel errors for known failure modes in the domain layer.
var (
	// ErrUserNotFound is returned when a GitLab username does not resolve
	// to any user (empty result array from the users search endpoint).
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidDateRange is returned when a date range has a from date
	// that is after its to date.
	ErrInvalidDateRange = errors.New("date range: from is after to")
)
