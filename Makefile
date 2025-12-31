SHELL := /bin/bash
GOFLAGS ?= -mod=vendor

.PHONY: tidy vendor test build-profctl build-server run-server install-profctl clean

tidy:
	go mod tidy

vendor: tidy
	go mod vendor

test:
	GOFLAGS='$(GOFLAGS)' go test ./...

build-profctl:
	GOFLAGS='$(GOFLAGS)' go build -o bin/profctl ./cmd/profctl

build-server:
	GOFLAGS='$(GOFLAGS)' go build -o bin/pprof-mcp-server ./cmd/pprof-mcp-server

run-server:
	GOFLAGS='$(GOFLAGS)' go run ./cmd/pprof-mcp-server

install-profctl:
	GOFLAGS='$(GOFLAGS)' go install ./cmd/profctl

clean:
	rm -rf bin
