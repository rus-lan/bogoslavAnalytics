package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// toolVersionFallback is the last-resort, hand-maintained version
// string ToolVersion falls back to when NEITHER automatic source below
// (toolVersionLdflags, runtime/debug.ReadBuildInfo) carries any VCS
// information at all: source with no .git present (an extracted module
// zip, a source tarball) and no -ldflags passed at build time either.
// It is deliberately NOT used for a dirty build (uncommitted changes on
// top of an otherwise-identified commit) -- see resolveToolVersion's
// doc comment for why that case gets a per-process nonce instead, never
// this constant.
//
// This is the one remaining place a release still needs a human to
// remember anything, and getting it wrong here is a much narrower miss
// than the constant this whole file replaces: it only matters for a
// `go build`/`go run` invoked directly against source with no VCS
// metadata and no Makefile involved -- which is not how this repo's own
// release process (`make dist`, `make build`) or an end-user `go
// install .../cmd/bogoslav-cli@vX.Y.Z` ever build a binary. Both of
// those are covered automatically instead; see resolveToolVersion.
const toolVersionFallback = "v0.2.1"

// toolVersionLdflags is set at link time by `make dist`/`make build`
// (Makefile) via `-ldflags "-X
// github.com/rus-lan/bogoslavAnalytics/internal/app.toolVersionLdflags=$(VERSION)"`,
// where $(VERSION) is `git describe --tags --always --dirty` -- computed
// once, automatically, by the Makefile itself, never by hand. Empty
// unless a build actually passes that flag: `go install
// module@version` does not run this repo's Makefile at all, and a
// plain `go build`/`go run` invoked directly does not either.
//
// This has to be a package-level string var, not the more usual const:
// -ldflags -X can only overwrite a var's initial value at link time, by
// patching the binary's data section after compilation -- a const has
// no storage to patch, its value is baked into every place it is used
// at compile time.
var toolVersionLdflags string

// ToolVersion is this binary's own version, folded into
// cache.QueryHash (and, since the fix this comment documents, into
// every other artifact/value cache key that is sensitive to how this
// tool builds its GitLab requests or interprets their results) so a
// cache key changes across a release that changed either (TZ.md
// section 4.6). It is resolved once, at package init, by
// resolveToolVersion -- see that function's doc comment for the full
// precedence, the evidence behind each source, and why a dirty or
// unidentifiable build never gets to reuse another build's cache
// entries.
var ToolVersion = resolveToolVersion(toolVersionLdflags, readMainModuleVersion())

// readMainModuleVersion returns runtime/debug.ReadBuildInfo's own
// verdict on this binary's main module version, or "" if build info is
// unavailable at all for this binary (only true for a binary built with
// `-buildinfo=false`; every build this repo's Makefile or a plain `go
// build`/`go install` produces carries build info).
func readMainModuleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// resolveToolVersion picks ToolVersion's value. Two automatic sources
// are tried first, in this order, each used as-is the moment it
// resolves to a "clean" version (defined below):
//
//  1. ldflags: the exact string `make dist`/`make build` baked in via
//     -ldflags -X, from `git describe --tags --always --dirty`
//     (Makefile). This is the only source needing the build process
//     itself to cooperate -- and, because it does, it is fully
//     automatic for this repo's own release path: nobody has to
//     remember to bump anything, the Makefile always recomputes it
//     from the tag actually being built.
//
//  2. buildInfoVersion: runtime/debug.ReadBuildInfo's own module
//     version for this binary's main module. This is NOT redundant
//     with ldflags -- it covers exactly the path ldflags cannot reach,
//     `go install .../cmd/bogoslav-cli@vX.Y.Z`, which never runs this
//     repo's Makefile and so never gets an -ldflags -X of any kind.
//     Verified empirically, not assumed: installing this exact module
//     at its real v0.2.1 tag (`go install
//     github.com/rus-lan/bogoslavAnalytics/cmd/bogoslav-cli@v0.2.1`,
//     network-fetched from the real upstream; the module cache holds
//     no .git at all for the extracted copy) and inspecting the result
//     with `go version -m` shows `mod
//     github.com/rus-lan/bogoslavAnalytics v0.2.1` -- ReadBuildInfo
//     genuinely returns the exact tag `go install` was asked for.
//
//     A second, unplanned-for finding from that same verification pass,
//     also empirical, not assumed: on this Go toolchain (go1.26.3),
//     ReadBuildInfo does NOT always read "(devel)" for a plain `go
//     build` the way an older belief (recorded in this function's git
//     history) assumed -- it only does when there is no VCS metadata at
//     all next to the source being built (confirmed by building
//     directly out of the module-cache copy above, which has no .git:
//     `go version -m` on that binary shows literally `(devel)`, and the
//     same is true even for a git repo with commits but zero tags at
//     all -- verified separately). When a plain `go build` (including
//     `make dist`'s cross-compiled, -trimpath, -ldflags="-s -w" build,
//     tested with the exact same flags) runs INSIDE an actual git
//     checkout, Go's own VCS build-info stamping (default since Go
//     1.18, -buildvcs=auto) derives a real version from the checkout's
//     git state on its own, with no -ldflags at all: an exact clean
//     checkout at a tag reads the bare tag ("v0.2.1"); a clean checkout
//     one commit past the last tag reads a standard Go pseudo-version
//     ("v0.2.2-0.20260716030242-cdc042383599" in the verification run);
//     either state with uncommitted changes appends git's own dirty
//     marker ("v0.2.1+dirty"). "Uncommitted changes" here is stricter
//     than `git describe --dirty`'s own definition, also verified, not
//     assumed: a tree with a stray UNTRACKED file and nothing else
//     changed already reads "+dirty" through ReadBuildInfo, while `git
//     describe --dirty` on that same tree stays clean (it only looks at
//     tracked-file modifications). resolveToolVersion does not need
//     both sources to agree on what counts as dirty -- see "clean",
//     below -- so this asymmetry only ever makes the overall result MORE
//     conservative (an untracked stray file can push buildInfoVersion
//     into the unstable branch even when ldflags's own git-describe
//     value would not have flagged it), never less. None of this
//     changes which source resolveToolVersion prefers -- ldflags still
//     wins when present, and does not depend on .git being there at all,
//     which is the more robust property for a release build -- but it
//     does mean a clean, fully-committed local `go build` during
//     development already gets a real, correctly-changing-per-commit
//     version through this same path, without needing -ldflags.
//
// "Clean" excludes an empty string (the source did not resolve at all)
// and the literal "(devel)" (no VCS metadata present for ReadBuildInfo
// to stamp) from being used as-is; a value carrying git's own dirty
// marker (git describe's "-dirty" suffix in the ldflags source, Go's
// own build-info "+dirty" suffix in the ReadBuildInfo source) is ALSO
// excluded, but handled differently from the other two -- see below.
//
// If neither source resolved cleanly:
//
//   - If either one came back dirty (uncommitted local changes on top
//     of an otherwise-identified commit): resolveToolVersion returns a
//     version unique to THIS process (unstableToolVersion), never
//     toolVersionFallback. A dirty marker is the same regardless of
//     WHAT is uncommitted, so two different local edits -- one with a
//     bug, one with the fix -- would otherwise resolve to the exact
//     same "version" and silently share cache entries: precisely the
//     failure this whole mechanism exists to rule out (TZ.md section
//     4.6's v0.2.0 incident, one level down -- an old uncommitted
//     edit's cache answering a new uncommitted edit's query, instead of
//     an old release's cache answering a new release's). The constant
//     cannot help here: it is a string compiled into the binary that
//     only changes when a human edits it, so it stays identical across
//     every dirty rebuild regardless of what changed.
//
//   - Otherwise (no VCS information at all, from either source: both
//     empty, or "(devel)", never dirty): resolveToolVersion falls back
//     to toolVersionFallback, the hand-maintained constant. This case
//     is different in kind from the dirty one above: with genuinely no
//     VCS information available, the constant IS still a meaningful
//     signal, because it is itself source code -- checked into the repo
//     and bumped by hand at each real release, exactly like the
//     original single-constant design this file replaces -- so two
//     different tagged snapshots built this way (each carrying its OWN
//     historical toolVersionFallback line) still resolve to two
//     different strings, as long as a human keeps bumping it for this
//     one narrow path. That is the one place this design still asks a
//     human to remember something, and it is confined to a build with
//     no VCS metadata and no -ldflags -- not this repo's own release
//     path, and not `go install .../cmd/bogoslav-cli@vX.Y.Z` either.
//
// A process-unique fallback (the dirty case) still lets every artifact
// one process writes share a cache key with every other artifact from
// the SAME process (repeated calls inside one bogoslav-mcp session, or
// within one bogoslav-cli invocation, are still consistent with
// themselves), but no artifact from THIS process is ever mistaken for a
// fresh hit by any OTHER process, dev-rebuilt or not -- which is the
// concrete answer to "what should a dirty/unidentifiable dev build mean
// for the cache": never trusted across a rebuild or a second run.
func resolveToolVersion(ldflags, buildInfoVersion string) string {
	if v := cleanResolvedVersion(ldflags); v != "" {
		return v
	}
	if v := cleanResolvedVersion(buildInfoVersion); v != "" {
		return v
	}
	if isDirtyVersion(ldflags) || isDirtyVersion(buildInfoVersion) {
		return unstableToolVersion()
	}
	return toolVersionFallback
}

// cleanResolvedVersion returns v unchanged if it identifies one
// reproducible build, or "" if it does not (empty, "(devel)", or
// dirty) -- see resolveToolVersion's doc comment for how each of those
// is handled once none of the sources come back clean.
func cleanResolvedVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" || isDirtyVersion(v) {
		return ""
	}
	return v
}

// isDirtyVersion reports whether v carries git's own "uncommitted
// changes" marker: git describe's "-dirty" suffix (the ldflags source)
// or Go's own build-info "+dirty" suffix (the ReadBuildInfo source).
func isDirtyVersion(v string) bool {
	return strings.Contains(v, "-dirty") || strings.Contains(v, "+dirty")
}

// unstableToolVersion returns a version string unique to this process,
// for the dirty/unidentifiable case (resolveToolVersion's doc comment).
// Generated once, at package init (Go runs every package-level var
// initializer exactly once, before main, single threaded), and never
// written to disk anywhere itself -- only ever folded into cache keys
// the same way a real released ToolVersion is.
func unstableToolVersion() string {
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		// crypto/rand.Read failing means the OS's own randomness source
		// is broken -- effectively never in practice on Linux, macOS or
		// Windows. This fallback still keeps every artifact this one
		// process writes mutually consistent (resolveToolVersion runs
		// once per process either way); it only loses the "never
		// trusted by a second unidentifiable run" property for this one
		// process, a narrower miss than crashing at startup over it.
		return "dev-unstable"
	}
	return "dev-unstable-" + hex.EncodeToString(nonce)
}

// dirtyVersionNote explains, in plain words, what a "dev-unstable-*"
// ToolVersion means: a bare nonce (see unstableToolVersion) tells a
// reader nothing on its own -- it looks like an opaque ID, not like
// "this is not a release". This is the exact information the incident
// VersionString exists for was missing: a user reporting "v0.2.1"
// while actually running a different, unidentifiable build had no
// command to ask, and would have been no better off from a `version`
// command that answered with a bare hex string either.
const dirtyVersionNote = "note: this is an unreleased build from a dirty tree (uncommitted local " +
	"changes on top of a commit, or no VCS metadata at all) -- not a tagged release. The value above " +
	"is unique to this one running process; a rebuild from the exact same source gets a different one, " +
	"so do not compare it across runs or treat two matching values as proof of the same build."

// VersionString is the exact text every binary's `version` command (and
// `--version` flag, where offered) prints, byte for byte: bogoslav-cli,
// bogoslav-mcp and bogoslav-skills all call this one function, so
// running `version` on any of the three always answers with the exact
// same thing (TZ.md section 7.4) -- the whole point being that a report
// of "I'm on v0.2.1" is something any of the three binaries can be asked
// to confirm or deny for itself, in ten seconds, instead of `strings
// $(command -v bogoslav-cli) | grep bogoslavAnalytics`.
//
// Two lines always: the resolved ToolVersion, and the Go toolchain this
// binary was built with (runtime.Version(), e.g. "go1.25.0") -- useful
// on its own when a bug turns out to be toolchain-specific, and free to
// include since it needs no extra resolution work beyond what this
// process already has. A third line (dirtyVersionNote) is added only
// when ToolVersion is a dev-unstable-* nonce, so that value is never
// printed bare -- see dirtyVersionNote's own doc comment.
//
// Deliberately NOT included: a raw VCS revision/timestamp from
// runtime/debug.BuildInfo.Settings. A clean pseudo-version
// (resolveToolVersion's ldflags/buildInfoVersion "ahead of last tag"
// case) already embeds a 12-character commit hash in ToolVersion itself
// (Go's own pseudo-version format); an exact tag has a real release to
// look the commit up from; and a dirty build gets the nonce instead,
// precisely so its cache entries are never mistaken for another dirty
// build's -- surfacing the underlying revision there too would still
// leave two different dirty builds of the very same commit printing the
// same revision, undermining the one property that nonce exists for.
func VersionString() string {
	return formatVersionString(ToolVersion, runtime.Version())
}

// formatVersionString is VersionString's testable core, taking both
// inputs as plain parameters instead of reading the package-level
// ToolVersion var and calling runtime.Version() directly -- the same
// seam resolveToolVersion already uses, for the same reason: tests can
// exercise every case (a clean tag, a pseudo-version, an unstable dirty
// nonce) without needing to fake package init or the Go toolchain
// itself.
func formatVersionString(version, goVersion string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "version: %s\n", version)
	if isUnstableVersion(version) {
		b.WriteString(dirtyVersionNote)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "go: %s\n", goVersion)
	return b.String()
}

// isUnstableVersion reports whether v is one of unstableToolVersion's
// own values -- the "dev-unstable" prefix is unstableToolVersion's own
// literal, checked here by prefix rather than by trying to parse the
// hex nonce (or its absence, in the crypto/rand failure fallback) back
// out of it.
func isUnstableVersion(v string) bool {
	return strings.HasPrefix(v, "dev-unstable")
}
