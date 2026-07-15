.PHONY: build test lint fmt

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
