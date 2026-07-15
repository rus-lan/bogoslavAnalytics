package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/apps/internal/domain"
)

// smokeServer builds a fake GitLab server for the smoke test: it serves
// comment events for one user and /discussions for a fixed set of merge
// requests.
func smokeServer(t *testing.T, events []CommentEvent, discussionsByMR map[[2]int64][]domain.Discussion) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v4/users/42/events":
			if _, present := r.URL.Query()["target_type"]; present {
				t.Errorf("smoke test events request contains target_type = %v", r.URL.Query()["target_type"])
			}
			if r.URL.Query().Get("page") != "1" {
				writeJSON(t, w, []CommentEvent{})
				return
			}
			writeJSON(t, w, events)
		default:
			var projectID, mrIID int64
			if _, err := fmt.Sscanf(r.URL.Path, "/api/v4/projects/%d/merge_requests/%d/discussions", &projectID, &mrIID); err == nil {
				if r.URL.Query().Get("page") != "1" {
					writeJSON(t, w, []domain.Discussion{})
					return
				}
				writeJSON(t, w, discussionsByMR[[2]int64{projectID, mrIID}])
				return
			}
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
	}))
}

func discussionWithNote(id int64, noteID int64, noteType domain.NoteType, authorID int64, system bool, createdAt time.Time) domain.Discussion {
	return domain.Discussion{
		ID:             fmt.Sprintf("disc-%d", id),
		IndividualNote: noteType == domain.NoteTypeNone,
		Notes: []domain.Note{{
			ID:           noteID,
			Type:         noteType,
			Body:         "body",
			Author:       domain.Author{ID: authorID, Username: "user"},
			CreatedAt:    createdAt,
			System:       system,
			NoteableID:   77,
			NoteableType: "MergeRequest",
			ProjectID:    123,
		}},
	}
}

func mrEvent(projectID, mrIID int64, createdAt time.Time) CommentEvent {
	return CommentEvent{
		ProjectID:  projectID,
		ActionName: "commented on",
		TargetType: "DiscussionNote",
		CreatedAt:  createdAt,
		Note: EventNote{
			System:       false,
			NoteableID:   77,
			NoteableType: "MergeRequest",
			NoteableIID:  mrIID,
		},
	}
}

func TestClient_SmokeTest_passedWhenEventCountMatchesOrExceedsDiscussionCount(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	replyAt := fixedNow.Add(-24 * time.Hour)

	events := []CommentEvent{
		mrEvent(123, 77, replyAt),
		mrEvent(123, 77, replyAt.Add(time.Minute)),
	}
	discussions := map[[2]int64][]domain.Discussion{
		{123, 77}: {
			discussionWithNote(1, 200, domain.NoteTypeDiscussion, 42, false, replyAt),
		},
	}

	srv := smokeServer(t, events, discussions)
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	result, err := c.SmokeTest(t.Context(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if result != domain.SmokePassed {
		t.Errorf("SmokeTest() = %q, want %q", result, domain.SmokePassed)
	}
}

func TestClient_SmokeTest_failedWhenEventsUndercountDiscussions(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	replyAt := fixedNow.Add(-24 * time.Hour)

	// Only one event reaches the client for this MR, but /discussions
	// shows two thread-reply notes from the user in the same window: the
	// events API is silently losing DiscussionNote replies.
	events := []CommentEvent{
		mrEvent(123, 77, replyAt),
	}
	discussions := map[[2]int64][]domain.Discussion{
		{123, 77}: {
			discussionWithNote(1, 200, domain.NoteTypeDiscussion, 42, false, replyAt),
			discussionWithNote(2, 201, domain.NoteTypeDiscussion, 42, false, replyAt.Add(time.Minute)),
		},
	}

	srv := smokeServer(t, events, discussions)
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	result, err := c.SmokeTest(t.Context(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if result != domain.SmokeFailed {
		t.Errorf("SmokeTest() = %q, want %q", result, domain.SmokeFailed)
	}
}

func TestClient_SmokeTest_unknownWhenNoThreadRepliesSampled(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	commentAt := fixedNow.Add(-24 * time.Hour)

	events := []CommentEvent{
		mrEvent(123, 77, commentAt),
	}
	// The only note the user has on this MR is a plain, non-threaded
	// comment (Type == NoteTypeNone), so the sample never sees a thread
	// reply.
	discussions := map[[2]int64][]domain.Discussion{
		{123, 77}: {
			discussionWithNote(1, 200, domain.NoteTypeNone, 42, false, commentAt),
		},
	}

	srv := smokeServer(t, events, discussions)
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	result, err := c.SmokeTest(t.Context(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if result != domain.SmokeUnknown {
		t.Errorf("SmokeTest() = %q, want %q", result, domain.SmokeUnknown)
	}
}

func TestClient_SmokeTest_unknownWhenNoCandidateEventsAtAll(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)

	srv := smokeServer(t, []CommentEvent{}, nil)
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	result, err := c.SmokeTest(t.Context(), 42)
	if err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if result != domain.SmokeUnknown {
		t.Errorf("SmokeTest() = %q, want %q", result, domain.SmokeUnknown)
	}
}

func TestClient_SmokeTest_samplesAtMostFiveCandidates(t *testing.T) {
	fixedNow := time.Date(2026, time.July, 15, 12, 0, 0, 0, time.UTC)
	commentAt := fixedNow.Add(-24 * time.Hour)

	var events []CommentEvent
	discussions := map[[2]int64][]domain.Discussion{}
	for i := int64(1); i <= 8; i++ {
		events = append(events, mrEvent(123, i, commentAt))
		discussions[[2]int64{123, i}] = []domain.Discussion{
			discussionWithNote(i, 200+i, domain.NoteTypeDiscussion, 42, false, commentAt),
		}
	}

	var discussionRequests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/users/42/events" {
			if r.URL.Query().Get("page") != "1" {
				writeJSON(t, w, []CommentEvent{})
				return
			}
			writeJSON(t, w, events)
			return
		}
		var projectID, mrIID int64
		if _, err := fmt.Sscanf(r.URL.Path, "/api/v4/projects/%d/merge_requests/%d/discussions", &projectID, &mrIID); err == nil {
			if r.URL.Query().Get("page") == "1" {
				discussionRequests++
			}
			if r.URL.Query().Get("page") != "1" {
				writeJSON(t, w, []domain.Discussion{})
				return
			}
			writeJSON(t, w, discussions[[2]int64{projectID, mrIID}])
			return
		}
		t.Fatalf("unexpected request path: %s", r.URL.Path)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	if _, err := c.SmokeTest(t.Context(), 42); err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}
	if discussionRequests != smokeMaxCandidates {
		t.Errorf("discussion requests made = %d, want %d (smokeMaxCandidates)", discussionRequests, smokeMaxCandidates)
	}
}

// TestClient_SmokeTest_windowUsesUTCNotLocalNow pins c.now to an instant
// that has already rolled over to the next calendar day in a zone east of
// UTC, but is still the previous day in UTC. If the smoke window were built
// from the local year/month/day instead of the UTC one, the "before" query
// bound sent to the events endpoint would be shifted a day later than the
// correct UTC-derived window.
func TestClient_SmokeTest_windowUsesUTCNotLocalNow(t *testing.T) {
	east := time.FixedZone("UTC+14", 14*60*60)
	// 2026-07-15T00:30:00+14:00 is 2026-07-14T10:30:00Z: the local calendar
	// day is already the 15th, but the UTC calendar day is still the 14th.
	fixedNow := time.Date(2026, time.July, 15, 0, 30, 0, 0, east)

	var gotBefore, gotAfter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v4/users/42/events" {
			if r.URL.Query().Get("page") == "1" {
				gotBefore = r.URL.Query().Get("before")
				gotAfter = r.URL.Query().Get("after")
			}
			writeJSON(t, w, []CommentEvent{})
			return
		}
		t.Fatalf("unexpected request path: %s", r.URL.Path)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "token")
	c.now = func() time.Time { return fixedNow }

	if _, err := c.SmokeTest(t.Context(), 42); err != nil {
		t.Fatalf("SmokeTest() error = %v", err)
	}

	// Correct (UTC-derived) window: to = 2026-07-14, so before = to+1 day.
	wantBefore := "2026-07-15"
	// If the bug were present, "to" would be 2026-07-15 (the local day) and
	// before would come out as 2026-07-16 instead.
	if gotBefore != wantBefore {
		t.Errorf("events request before = %q, want %q (window must use UTC calendar day, not local)", gotBefore, wantBefore)
	}
	wantAfter := domain.NewDate(2026, time.July, 14).
		Start().AddDate(0, 0, -smokeWindowDays-1).Format("2006-01-02")
	if gotAfter != wantAfter {
		t.Errorf("events request after = %q, want %q", gotAfter, wantAfter)
	}
}

func TestMRCandidatesFromEvents_ignoresNonMergeRequestAndSystemEvents(t *testing.T) {
	createdAt := time.Now()
	events := []CommentEvent{
		mrEvent(1, 10, createdAt),
		{ProjectID: 1, Note: EventNote{NoteableType: "Issue", NoteableIID: 11}, CreatedAt: createdAt},
		{ProjectID: 1, Note: EventNote{NoteableType: "MergeRequest", NoteableIID: 12, System: true}, CreatedAt: createdAt},
		mrEvent(1, 10, createdAt),
	}
	candidates := mrCandidatesFromEvents(events, 5)
	if len(candidates) != 1 {
		t.Fatalf("candidates = %+v, want exactly 1 (project 1, mr 10)", candidates)
	}
	if candidates[0].projectID != 1 || candidates[0].mrIID != 10 || candidates[0].eventCount != 2 {
		t.Errorf("candidates[0] = %+v, want {projectID:1 mrIID:10 eventCount:2}", candidates[0])
	}
}

// ensure the json encoding round trip of CommentEvent matches the wire
// shape TZ.md documents (project_id, action_name, target_type, note.*).
func TestCommentEvent_unmarshalsDocumentedShape(t *testing.T) {
	raw := `{
		"project_id": 1,
		"action_name": "commented on",
		"target_type": "DiscussionNote",
		"created_at": "2026-03-01T10:00:00Z",
		"note": {"system": false, "noteable_id": 5, "noteable_type": "MergeRequest", "noteable_iid": 9}
	}`
	var e CommentEvent
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if e.ProjectID != 1 || e.Note.NoteableIID != 9 || e.Note.NoteableType != "MergeRequest" {
		t.Errorf("CommentEvent = %+v, want project_id=1 note.noteable_iid=9 note.noteable_type=MergeRequest", e)
	}
}
