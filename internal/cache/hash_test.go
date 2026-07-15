package cache

import (
	"testing"
	"time"

	"github.com/rus-lan/bogoslavAnalytics/internal/domain"
)

func TestCanonicalJSON_sortsKeysLexicographically(t *testing.T) {
	in := map[string]any{
		"b": 1,
		"a": 2,
		"m": map[string]any{
			"z": 1,
			"y": 2,
		},
	}
	want := `{"a":2,"b":1,"m":{"y":2,"z":1}}`

	got, err := canonicalJSON(in)
	if err != nil {
		t.Fatalf("canonicalJSON() error = %v", err)
	}
	if string(got) != want {
		t.Errorf("canonicalJSON() = %s, want %s", got, want)
	}
}

// buildQuery is the base normalized query fixture shared by the hash
// tests below.
func buildQuery() domain.Query {
	return domain.Query{
		GitlabURL: "https://gitlab.example.com",
		UserID:    42,
		From:      domain.NewDate(2026, time.January, 1),
		To:        domain.NewDate(2026, time.June, 30),
		MoreThan:  5,
		Group:     "my-group",
	}
}

// TestHash_deterministic_acrossMapInsertionOrder guards against exactly
// the failure mode TZ.md section 4.5 calls out: the same logical value,
// built as two maps with different insertion order, must hash
// identically. It runs many times to shake out Go's randomized map
// iteration order.
func TestHash_deterministic_acrossMapInsertionOrder(t *testing.T) {
	build := func(order []string) map[string]any {
		m := map[string]any{}
		for _, k := range order {
			switch k {
			case "a":
				m["a"] = int64(1)
			case "b":
				m["b"] = "two"
			case "c":
				m["c"] = []any{1, 2, 3}
			case "d":
				m["d"] = map[string]any{"y": 1, "x": 2}
			}
		}
		return m
	}

	orderA := []string{"a", "b", "c", "d"}
	orderB := []string{"d", "c", "b", "a"}

	want, err := Hash(build(orderA))
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}

	for i := 0; i < 200; i++ {
		got, err := Hash(build(orderB))
		if err != nil {
			t.Fatalf("Hash() error on iteration %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("Hash() nondeterministic on iteration %d: got %s, want %s", i, got, want)
		}
	}
}

// TestQueryHash_deterministic_acrossFieldOrder builds the same logical
// query via two struct literals with fields named in a different order
// and checks QueryHash agrees, repeatedly.
func TestQueryHash_deterministic_acrossFieldOrder(t *testing.T) {
	buildA := func() domain.Query {
		return domain.Query{
			GitlabURL: "https://gitlab.example.com",
			UserID:    42,
			From:      domain.NewDate(2026, time.January, 1),
			To:        domain.NewDate(2026, time.June, 30),
			MoreThan:  5,
			Group:     "my-group",
			Project:   "my-group/repo",
		}
	}
	buildB := func() domain.Query {
		return domain.Query{
			Project:   "my-group/repo",
			Group:     "my-group",
			MoreThan:  5,
			To:        domain.NewDate(2026, time.June, 30),
			From:      domain.NewDate(2026, time.January, 1),
			UserID:    42,
			GitlabURL: "https://gitlab.example.com",
		}
	}

	want, err := QueryHash(buildA())
	if err != nil {
		t.Fatalf("QueryHash() error = %v", err)
	}

	for i := 0; i < 200; i++ {
		got, err := QueryHash(buildB())
		if err != nil {
			t.Fatalf("QueryHash() error on iteration %d: %v", i, err)
		}
		if got != want {
			t.Fatalf("QueryHash() nondeterministic on iteration %d: got %s, want %s", i, got, want)
		}
	}
}

func TestQueryHash_sensitiveToChanges(t *testing.T) {
	base := buildQuery()
	baseHash, err := QueryHash(base)
	if err != nil {
		t.Fatalf("QueryHash(base) error = %v", err)
	}

	cases := []struct {
		name  string
		build func() domain.Query
	}{
		{
			name: "more_than changes",
			build: func() domain.Query {
				q := buildQuery()
				q.MoreThan = 6
				return q
			},
		},
		{
			name: "from changes",
			build: func() domain.Query {
				q := buildQuery()
				q.From = domain.NewDate(2026, time.February, 1)
				return q
			},
		},
		{
			name: "to changes",
			build: func() domain.Query {
				q := buildQuery()
				q.To = domain.NewDate(2026, time.July, 31)
				return q
			},
		},
		{
			name: "gitlab_url changes",
			build: func() domain.Query {
				q := buildQuery()
				q.GitlabURL = "https://gitlab.other.example.com"
				return q
			},
		},
		{
			name: "user_id changes",
			build: func() domain.Query {
				q := buildQuery()
				q.UserID = 43
				return q
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := QueryHash(tc.build())
			if err != nil {
				t.Fatalf("QueryHash() error = %v", err)
			}
			if got == baseHash {
				t.Errorf("QueryHash() = %s, want different hash than base %s", got, baseHash)
			}
		})
	}
}

// TestQueryHash_omittedOptionalParams encodes the rule this package
// implements for domain.Query (see QueryHash's doc comment):
//   - group/project are plain strings, so an omitted value and an
//     explicit empty string are the same Go zero value and hash the
//     same (both are absent keys).
//   - mr is a pointer, so a nil mr (omitted) and a non-nil mr pointing
//     at 0 (explicitly set) are distinguishable and hash differently.
func TestQueryHash_omittedOptionalParams(t *testing.T) {
	t.Run("group omitted equals group explicitly empty", func(t *testing.T) {
		omitted := buildQuery()
		omitted.Group = ""
		explicitEmpty := buildQuery()
		explicitEmpty.Group = ""

		got1, err := QueryHash(omitted)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		got2, err := QueryHash(explicitEmpty)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		if got1 != got2 {
			t.Errorf("QueryHash() = %s and %s, want equal", got1, got2)
		}
	})

	t.Run("group set changes hash versus omitted", func(t *testing.T) {
		omitted := buildQuery()
		omitted.Group = ""
		set := buildQuery()
		set.Group = "my-group"

		gotOmitted, err := QueryHash(omitted)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		gotSet, err := QueryHash(set)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		if gotOmitted == gotSet {
			t.Errorf("QueryHash() = %s, want different hash when group is set", gotSet)
		}
	})

	t.Run("mr nil differs from mr explicitly zero", func(t *testing.T) {
		nilMR := buildQuery()
		nilMR.MR = nil

		zero := int64(0)
		explicitZeroMR := buildQuery()
		explicitZeroMR.MR = &zero

		gotNil, err := QueryHash(nilMR)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		gotZero, err := QueryHash(explicitZeroMR)
		if err != nil {
			t.Fatalf("QueryHash() error = %v", err)
		}
		if gotNil == gotZero {
			t.Errorf("QueryHash() = %s, want different hash for nil mr versus explicit mr=0", gotZero)
		}
	})
}

// TestQueryHash_excludesStrategyAndSmoke checks the interpretation this
// package implements: the same request hashes identically whether or
// not Strategy/Smoke happen to be populated on the Query passed in,
// since a cache lookup runs before either is known (TZ.md sections 4.5,
// 5.3, 5.5).
func TestQueryHash_excludesStrategyAndSmoke(t *testing.T) {
	unresolved := buildQuery()

	resolved := buildQuery()
	resolved.Strategy = domain.StrategyEvents
	resolved.Smoke = domain.SmokePassed

	gotUnresolved, err := QueryHash(unresolved)
	if err != nil {
		t.Fatalf("QueryHash() error = %v", err)
	}
	gotResolved, err := QueryHash(resolved)
	if err != nil {
		t.Fatalf("QueryHash() error = %v", err)
	}
	if gotUnresolved != gotResolved {
		t.Errorf("QueryHash() = %s and %s, want equal regardless of Strategy/Smoke", gotUnresolved, gotResolved)
	}
}
