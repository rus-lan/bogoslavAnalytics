.PHONY: build test lint fmt contracts

build:
	mkdir -p bin
	go build -o bin/bogoslav-cli ./cmd/bogoslav-cli
	go build -o bin/bogoslav-mcp ./cmd/bogoslav-mcp
	go build -o bin/bogoslav-skills ./cmd/bogoslav-skills

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w .

# contracts regenerates ../contracts/openapi.yaml from the Go types in
# internal/mcptool and internal/artifact (TZ.md section 10). Run this
# after changing any of those types and commit the result -- `go test
# ./internal/contracts/...` fails if the committed file drifts from
# what this target produces.
contracts:
	go run ./cmd/gen-contracts -out ../contracts/openapi.yaml
