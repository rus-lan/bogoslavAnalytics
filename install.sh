#!/bin/sh
# install.sh -- installs bogoslav-cli, bogoslav-mcp and bogoslav-skills
# from a GitHub release, no Go toolchain required.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/rus-lan/bogoslavAnalytics/main/install.sh | sh
#
# Honesty about curl|sh:
#   This pipes a remote script straight into your shell. Read it first
#   instead of trusting it blind:
#     curl -fsSL https://raw.githubusercontent.com/rus-lan/bogoslavAnalytics/main/install.sh | less
#   The SHA-256 verification this script does covers the downloaded
#   BINARIES, not this script itself -- there is no signature on the
#   script, so a compromised raw.githubusercontent.com or a MITM on
#   your network could still change what you run. Piping to a shell is
#   a convenience, not a security boundary.
#   For CI: pin BOGOSLAV_VERSION (e.g. BOGOSLAV_VERSION=v0.2.0) instead
#   of tracking latest. A pinned version makes the build reproducible;
#   tracking latest means a new release can silently change what your
#   pipeline runs.
#
# Environment variables:
#   BOGOSLAV_VERSION      version to install, e.g. "v0.2.0" (used as the
#                         release tag verbatim, no prefix added).
#                         Default: resolve the latest GitHub release.
#   BOGOSLAV_INSTALL_DIR  where to put the binaries.
#                         Default: "$HOME/.local/bin".
#   BOGOSLAV_BINS         space-separated subset of the three binaries
#                         to install, e.g. "bogoslav-cli bogoslav-mcp".
#                         Default: all three.
#   BOGOSLAV_ALLOW_NO_CHECKSUM
#                         if "1", install even when neither sha256sum
#                         nor shasum is on PATH (see note below).
#                         Default: unset, i.e. refuse.
#   BOGOSLAV_BASE_URL     override the "https://github.com/<repo>" base
#                         (used for testing against a mirror or a local
#                         fake release layout). Default:
#                         "https://github.com/rus-lan/bogoslavAnalytics".
#
# Checksum policy: this script refuses to install if it cannot verify
# a SHA-256 checksum, unless BOGOSLAV_ALLOW_NO_CHECKSUM=1 is set. Both
# sha256sum and shasum are present on essentially every Linux and
# macOS system including busybox, so refusing by default costs almost
# nobody anything, and a partial, unverified install is worse than a
# script that stops and says so.

set -eu

REPO="rus-lan/bogoslavAnalytics"
BASE_URL="${BOGOSLAV_BASE_URL:-https://github.com/${REPO}}"
INSTALL_DIR="${BOGOSLAV_INSTALL_DIR:-${HOME:-}/.local/bin}"
ALL_BINS="bogoslav-cli bogoslav-mcp bogoslav-skills"
BINS="${BOGOSLAV_BINS:-${ALL_BINS}}"

log() {
	printf '%s\n' "$*" >&2
}

die() {
	printf 'install.sh: error: %s\n' "$*" >&2
	exit 1
}

if [ -z "${HOME:-}" ] && [ -z "${BOGOSLAV_INSTALL_DIR:-}" ]; then
	die "\$HOME is not set and BOGOSLAV_INSTALL_DIR was not given; set BOGOSLAV_INSTALL_DIR explicitly"
fi

for want in $BINS; do
	case " $ALL_BINS " in
	*" $want "*) ;;
	*) die "unknown binary '$want' in BOGOSLAV_BINS; valid names: $ALL_BINS" ;;
	esac
done

HAVE_CURL=0
HAVE_WGET=0
if command -v curl >/dev/null 2>&1; then
	HAVE_CURL=1
elif command -v wget >/dev/null 2>&1; then
	HAVE_WGET=1
fi
if [ "$HAVE_CURL" = 0 ] && [ "$HAVE_WGET" = 0 ]; then
	die "neither curl nor wget found on PATH; install one of them and retry"
fi

HAVE_SHA256SUM=0
HAVE_SHASUM=0
if command -v sha256sum >/dev/null 2>&1; then
	HAVE_SHA256SUM=1
elif command -v shasum >/dev/null 2>&1; then
	HAVE_SHASUM=1
fi
if [ "$HAVE_SHA256SUM" = 0 ] && [ "$HAVE_SHASUM" = 0 ]; then
	if [ "${BOGOSLAV_ALLOW_NO_CHECKSUM:-}" = "1" ]; then
		log "warning: neither sha256sum nor shasum found; installing WITHOUT checksum verification (BOGOSLAV_ALLOW_NO_CHECKSUM=1)"
	else
		die "neither sha256sum nor shasum found on PATH; refusing to install unverified binaries. Set BOGOSLAV_ALLOW_NO_CHECKSUM=1 to override."
	fi
fi

UNAME_S=$(uname -s)
UNAME_M=$(uname -m)

case "$UNAME_S" in
Linux) OS=linux ;;
Darwin) OS=darwin ;;
*) die "unsupported OS '$UNAME_S' (only Linux and Darwin are supported)" ;;
esac

case "$UNAME_M" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*) die "unsupported architecture '$UNAME_M' (only x86_64/amd64 and aarch64/arm64 are supported)" ;;
esac

fetch_to_stdout() {
	# fetch_to_stdout <url> -- prints the body to stdout, no redirect
	# following (used only for resolving the "latest" tag from the
	# 302 Location header; a HEAD-shaped request is enough).
	url="$1"
	if [ "$HAVE_CURL" = 1 ]; then
		curl -sS -D - -o /dev/null "$url" 2>/dev/null || true
	else
		wget -S -q -O /dev/null "$url" 2>&1 || true
	fi
}

fetch_to_file() {
	# fetch_to_file <url> <dest> -- downloads url to dest, following
	# redirects, failing loudly on any non-2xx response.
	url="$1"
	dest="$2"
	if [ "$HAVE_CURL" = 1 ]; then
		curl -fsSL -o "$dest" "$url"
	else
		wget -q -O "$dest" "$url"
	fi
}

resolve_latest_tag() {
	# GitHub's "releases/latest" redirects to "releases/tag/<tag>".
	# Reading that Location header resolves "latest" to a concrete
	# tag without the GitHub API and without jq, which a minimal CI
	# image may not have.
	latest_url="${BASE_URL}/releases/latest"
	headers=$(fetch_to_stdout "$latest_url")
	loc=$(printf '%s\n' "$headers" | grep -i '^ *location:' | tail -1 | tr -d '\r')
	tag=${loc##*/releases/tag/}
	if [ -z "$tag" ] || [ "$tag" = "$loc" ]; then
		die "could not resolve the latest release from ${latest_url}; set BOGOSLAV_VERSION explicitly (e.g. BOGOSLAV_VERSION=v0.2.0)"
	fi
	printf '%s' "$tag"
}

if [ -n "${BOGOSLAV_VERSION:-}" ]; then
	TAG="$BOGOSLAV_VERSION"
else
	log "BOGOSLAV_VERSION not set, resolving latest release (for CI, pin a version instead)"
	TAG=$(resolve_latest_tag)
fi

log "installing bogoslav (tag $TAG) for ${OS}/${ARCH}"

TMP_DIR=""
cleanup() {
	[ -n "$TMP_DIR" ] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM
TMP_DIR=$(mktemp -d)

RELEASE_URL="${BASE_URL}/releases/download/${TAG}"

fetch_to_file "${RELEASE_URL}/SHA256SUMS" "${TMP_DIR}/SHA256SUMS"

verify_checksum() {
	# verify_checksum <asset-name> <path>
	name="$1"
	path="$2"
	expected=$(grep " $name\$" "${TMP_DIR}/SHA256SUMS" | awk '{print $1}')
	if [ -z "$expected" ]; then
		die "no checksum entry for $name in SHA256SUMS"
	fi
	if [ "$HAVE_SHA256SUM" = 1 ]; then
		actual=$(sha256sum "$path" | awk '{print $1}')
	elif [ "$HAVE_SHASUM" = 1 ]; then
		actual=$(shasum -a 256 "$path" | awk '{print $1}')
	else
		return 0
	fi
	if [ "$expected" != "$actual" ]; then
		die "checksum mismatch for $name: expected $expected, got $actual"
	fi
}

mkdir -p "$INSTALL_DIR"

INSTALLED=""
for bin in $BINS; do
	asset="${bin}_${OS}_${ARCH}"
	dest="${TMP_DIR}/${asset}"
	fetch_to_file "${RELEASE_URL}/${asset}" "$dest"
	verify_checksum "$asset" "$dest"
	chmod +x "$dest"
	mv -f "$dest" "${INSTALL_DIR}/${bin}"
	INSTALLED="${INSTALLED} ${bin}"
done

log ""
log "installed:${INSTALLED}"
log "into: ${INSTALL_DIR}"

case ":${PATH}:" in
*":${INSTALL_DIR}:"*) ;;
*) log "note: ${INSTALL_DIR} is not on your PATH; add it, e.g.: export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
esac
