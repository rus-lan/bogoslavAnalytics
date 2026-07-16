package app

import (
	"runtime"
	"strings"
	"testing"
)

// TestResolveToolVersion_precedence exercises resolveToolVersion's own
// documented three-source order directly, through the seam its two
// explicit parameters provide -- no linker or runtime/debug involved,
// just the pure decision function both real sources feed into.
func TestResolveToolVersion_precedence(t *testing.T) {
	cases := []struct {
		name      string
		ldflags   string
		buildInfo string
		wantExact string
	}{
		{
			name:      "ldflags wins over a clean buildInfo",
			ldflags:   "v0.3.0",
			buildInfo: "v0.2.9",
			wantExact: "v0.3.0",
		},
		{
			name:      "ldflags empty falls back to buildInfo",
			ldflags:   "",
			buildInfo: "v0.2.2-0.20260716030242-cdc042383599",
			wantExact: "v0.2.2-0.20260716030242-cdc042383599",
		},
		{
			name:      "ldflags is literal (devel) falls back to buildInfo",
			ldflags:   "(devel)",
			buildInfo: "v0.2.1",
			wantExact: "v0.2.1",
		},
		{
			name:      "ldflags carries git's -dirty suffix falls back to buildInfo",
			ldflags:   "v0.2.1-dirty",
			buildInfo: "v0.2.1",
			wantExact: "v0.2.1",
		},
		{
			name:      "both empty falls back to fallback constant",
			ldflags:   "",
			buildInfo: "",
			wantExact: toolVersionFallback,
		},
		{
			name:      "both literal (devel), no dirty marker anywhere, falls back to fallback constant",
			ldflags:   "(devel)",
			buildInfo: "(devel)",
			wantExact: toolVersionFallback,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveToolVersion(tc.ldflags, tc.buildInfo)
			if got != tc.wantExact {
				t.Errorf("resolveToolVersion(%q, %q) = %q, want %q", tc.ldflags, tc.buildInfo, got, tc.wantExact)
			}
		})
	}
}

// TestResolveToolVersion_devUnstableCases is split out from the table
// above because a dirty resolution resolves to a per-process-random
// value, not a fixed one to compare against with ==: this checks the
// *properties* resolveToolVersion documents for that case instead (its
// own doc comment's "never trusted across a rebuild or a second run")
// -- non-empty, and different across two separate calls. Every case
// here has at least one dirty marker present (never plain "(devel)"/
// empty on both sides -- that combination has its own fixed
// toolVersionFallback expectation in the precedence table above).
func TestResolveToolVersion_devUnstableCases(t *testing.T) {
	cases := []struct {
		name      string
		ldflags   string
		buildInfo string
	}{
		{name: "ldflags dirty (git describe style), buildInfo devel", ldflags: "v0.2.1-dirty", buildInfo: "(devel)"},
		{name: "ldflags empty, buildInfo dirty (go pseudo-version style)", ldflags: "", buildInfo: "v0.2.1+dirty"},
		{name: "both dirty", ldflags: "v0.2.1-dirty", buildInfo: "v0.2.1+dirty"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := resolveToolVersion(tc.ldflags, tc.buildInfo)
			b := resolveToolVersion(tc.ldflags, tc.buildInfo)
			if a == "" {
				t.Fatal("resolveToolVersion() = \"\", want a non-empty unstable marker")
			}
			if a == b {
				t.Errorf("resolveToolVersion() returned the same value %q twice for an unidentifiable build; want a fresh per-call marker so two different unreproducible builds never share a cache key", a)
			}
		})
	}
}

func TestCleanResolvedVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "devel", in: "(devel)", want: ""},
		{name: "git describe dirty suffix", in: "v0.2.1-dirty", want: ""},
		{name: "go pseudo-version dirty suffix", in: "v0.2.1+dirty", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		{name: "clean tag", in: "v0.2.1", want: "v0.2.1"},
		{name: "clean pseudo-version", in: "v0.2.2-0.20260716030242-cdc042383599", want: "v0.2.2-0.20260716030242-cdc042383599"},
		{name: "trims surrounding whitespace", in: "  v0.2.1  ", want: "v0.2.1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cleanResolvedVersion(tc.in)
			if got != tc.want {
				t.Errorf("cleanResolvedVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsDirtyVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "clean tag", in: "v0.2.1", want: false},
		{name: "empty", in: "", want: false},
		{name: "devel", in: "(devel)", want: false},
		{name: "git describe dirty suffix", in: "v0.2.1-dirty", want: true},
		{name: "git describe dirty suffix with commits ahead", in: "v0.2.1-1-gcdc0423-dirty", want: true},
		{name: "go pseudo-version dirty suffix", in: "v0.2.1+dirty", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isDirtyVersion(tc.in); got != tc.want {
				t.Errorf("isDirtyVersion(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestUnstableToolVersion_neverEmptyAndVariesPerCall checks the two
// properties resolveToolVersion's fallback relies on: unstableToolVersion
// never returns "", and two calls never collide (the whole point of
// folding in a fresh random nonce every time).
func TestUnstableToolVersion_neverEmptyAndVariesPerCall(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		v := unstableToolVersion()
		if v == "" {
			t.Fatalf("unstableToolVersion() call %d = \"\", want non-empty", i)
		}
		if seen[v] {
			t.Fatalf("unstableToolVersion() call %d repeated a previous value %q", i, v)
		}
		seen[v] = true
	}
}

// TestReadMainModuleVersion_returnsSomething is a light sanity check,
// not a behavior pin: on any Go toolchain that supports
// runtime/debug.ReadBuildInfo (every supported one), a test binary
// always has build info, so this must never return "" for THIS
// process. What exact string it returns depends on the environment
// this test runs in (VCS presence/cleanliness) and is covered instead
// by the empirical evidence in resolveToolVersion's doc comment and
// this task's own verification transcript, not asserted here as a
// fixed value.
func TestReadMainModuleVersion_returnsSomething(t *testing.T) {
	if v := readMainModuleVersion(); v == "" {
		t.Error("readMainModuleVersion() = \"\", want a non-empty value for a normal go test binary")
	}
}

// TestFormatVersionString_cleanVersion proves a clean (tagged or
// pseudo-version) ToolVersion renders as just the version and Go
// toolchain lines, with no dirtyVersionNote -- that note only belongs
// next to a dev-unstable-* nonce (TestFormatVersionString_unstableVersion).
func TestFormatVersionString_cleanVersion(t *testing.T) {
	got := formatVersionString("v0.2.1", "go1.25.0")
	want := "version: v0.2.1\ngo: go1.25.0\n"
	if got != want {
		t.Errorf("formatVersionString(%q, %q) = %q, want %q", "v0.2.1", "go1.25.0", got, want)
	}
}

// TestFormatVersionString_unstableVersion proves a dev-unstable-* nonce
// is never printed bare: it always comes with dirtyVersionNote's
// explanation, right there in the same output, not left for the user to
// go find documentation for it.
func TestFormatVersionString_unstableVersion(t *testing.T) {
	nonce := "dev-unstable-3f9a2b1c4d5e6f70"
	got := formatVersionString(nonce, "go1.25.0")

	if !strings.HasPrefix(got, "version: "+nonce+"\n") {
		t.Fatalf("formatVersionString(%q, ...) = %q, want it to start with the version line", nonce, got)
	}
	if !strings.Contains(got, dirtyVersionNote) {
		t.Errorf("formatVersionString(%q, ...) = %q, want it to contain dirtyVersionNote explaining what the nonce means", nonce, got)
	}
	if !strings.HasSuffix(got, "go: go1.25.0\n") {
		t.Errorf("formatVersionString(%q, ...) = %q, want it to end with the go toolchain line", nonce, got)
	}
}

// TestIsUnstableVersion checks the one prefix isUnstableVersion actually
// keys off of, both ways.
func TestIsUnstableVersion(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "clean tag", in: "v0.2.1", want: false},
		{name: "clean pseudo-version", in: "v0.2.2-0.20260716030242-cdc042383599", want: false},
		{name: "unstable nonce", in: "dev-unstable-3f9a2b1c4d5e6f70", want: true},
		{name: "unstable fallback with no nonce", in: "dev-unstable", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isUnstableVersion(tc.in); got != tc.want {
				t.Errorf("isUnstableVersion(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestVersionString_containsToolVersion is VersionString's own sanity
// check: whatever this test process's own ToolVersion resolved to (a
// tag, a pseudo-version, or a dev-unstable-* nonce -- see
// resolveToolVersion's doc comment for why that depends on this
// environment), VersionString's output always contains it verbatim,
// since it is the one piece of information every version command
// exists to answer.
func TestVersionString_containsToolVersion(t *testing.T) {
	got := VersionString()
	if !strings.Contains(got, ToolVersion) {
		t.Errorf("VersionString() = %q, want it to contain ToolVersion %q", got, ToolVersion)
	}
	if !strings.Contains(got, runtime.Version()) {
		t.Errorf("VersionString() = %q, want it to contain the Go toolchain version %q", got, runtime.Version())
	}
}
