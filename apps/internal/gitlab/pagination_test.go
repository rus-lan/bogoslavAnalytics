package gitlab

import (
	"context"
	"errors"
	"testing"
)

func TestPaginate_walksPagesUntilShortPage(t *testing.T) {
	pages := [][]int{
		make([]int, perPage),
		make([]int, perPage),
		{1, 2, 3},
	}
	var requestedPages []int

	got, err := paginate(t.Context(), func(_ context.Context, page int) ([]int, error) {
		requestedPages = append(requestedPages, page)
		if page > len(pages) {
			t.Fatalf("paginate() requested page %d beyond the fixture (%d pages)", page, len(pages))
		}
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatalf("paginate() error = %v", err)
	}

	wantLen := perPage + perPage + 3
	if len(got) != wantLen {
		t.Errorf("paginate() returned %d items, want %d", len(got), wantLen)
	}
	wantPages := []int{1, 2, 3}
	if len(requestedPages) != len(wantPages) {
		t.Fatalf("requested pages = %v, want %v", requestedPages, wantPages)
	}
	for i, p := range wantPages {
		if requestedPages[i] != p {
			t.Errorf("requested pages = %v, want %v", requestedPages, wantPages)
			break
		}
	}
}

func TestPaginate_stopsAtPageLimitWithoutPagingDeeper(t *testing.T) {
	var maxRequestedPage int

	got, err := paginate(t.Context(), func(_ context.Context, page int) ([]int, error) {
		if page > maxRequestedPage {
			maxRequestedPage = page
		}
		// Always return a full page: the listing never ends.
		return make([]int, perPage), nil
	})
	if !errors.Is(err, ErrPageLimitReached) {
		t.Fatalf("paginate() error = %v, want ErrPageLimitReached", err)
	}
	if maxRequestedPage != maxPage {
		t.Errorf("paginate() requested up to page %d, want exactly %d (no deeper paging)", maxRequestedPage, maxPage)
	}
	if len(got) != maxPage*perPage {
		t.Errorf("paginate() returned %d items, want %d", len(got), maxPage*perPage)
	}
}

func TestPaginate_propagatesFetchError(t *testing.T) {
	wantErr := errors.New("boom")
	_, err := paginate(t.Context(), func(_ context.Context, page int) ([]int, error) {
		if page == 1 {
			// A full page so paginate keeps going instead of stopping on
			// a short page.
			return make([]int, perPage), nil
		}
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Errorf("paginate() error = %v, want %v", err, wantErr)
	}
}
