.PHONY: build test lint fmt contracts dist

build:
	mkdir -p bin
	go build -o bin/bogoslav-cli ./cmd/bogoslav-cli
	go build -o bin/bogoslav-mcp ./cmd/bogoslav-mcp
	go build -o bin/bogoslav-skills ./cmd/bogoslav-skills

# dist cross-compiles the three CLI binaries for every platform the
# curl|sh installer supports and writes plain (uncompressed, untarred)
# binaries plus a SHA256SUMS file into dist/.
#
# Naming: <binary>_<os>_<arch>, e.g. bogoslav-cli_linux_amd64. This is
# unambiguous (binary names use hyphens, the separator is underscore)
# and matches the goreleaser convention, so it reads naturally to
# anyone who has installed a Go tool before.
#
# Bare binaries, not tarballs: install.sh installs a subset of the
# three tools by default-all, override-some, and a bare binary means
# fetching a subset costs only the bytes of that subset. A tarball
# would force a full download (and a `tar` dependency on the install
# target) even to get one binary out of three. The cost is more
# release assets (12 files + checksums instead of 4 tarballs); that
# is a one-time build-time cost, not an install-time one.
#
# No windows/amd64: install.sh is POSIX sh and detects the platform
# via uname, which Windows does not provide outside WSL/MSYS/Git
# Bash. A curl|sh installer has no way to run on native Windows, so a
# windows/amd64 asset would sit unused in every release.
#
# CGO_ENABLED=0 gives static Linux binaries (verified: `file` and
# `ldd` on the linux/amd64 output show "statically linked", ldd
# reports "not a dynamic executable"), so they run in scratch/alpine
# CI images with no libc present. On Darwin, Go binaries always
# link against the system's libSystem.dylib regardless of CGO -- a
# fully static Mach-O binary is not a thing the platform allows -- so
# the Darwin outputs are dynamically linked against libSystem only,
# which is present on every macOS install by definition.
#
# -trimpath removes build-machine absolute paths from the binary so
# the same source built on two different machines/checkouts produces
# identical output. -ldflags="-s -w" drops the symbol table and DWARF
# debug info, cutting binary size roughly in half.
BINARIES := bogoslav-cli bogoslav-mcp bogoslav-skills
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
DIST := dist

dist:
	@mkdir -p $(DIST)
	@rm -f $(DIST)/*
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		for bin in $(BINARIES); do \
			out=$(DIST)/$${bin}_$${os}_$${arch}; \
			echo "building $$out"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o $$out ./cmd/$$bin; \
		done; \
	done
	@cd $(DIST) && (sha256sum * > SHA256SUMS 2>/dev/null || shasum -a 256 * > SHA256SUMS)
	@echo "wrote $(DIST)/SHA256SUMS"

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

# contracts regenerates contracts/openapi.yaml from the Go types in
# internal/mcptool and internal/artifact (TZ.md section 10). Run this
# after changing any of those types and commit the result -- `go test
# ./internal/contracts/...` fails if the committed file drifts from
# what this target produces.
contracts:
	go run ./cmd/gen-contracts -out contracts/openapi.yaml
