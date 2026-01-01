SHELL := /bin/bash
GOFLAGS ?= -mod=vendor

.PHONY: all tidy vendor test integration-test build-profctl build-server run-server install install-profctl install-server config clean

all: vendor test build-profctl build-server

tidy:
	go mod tidy

vendor: tidy
	go mod vendor

test:
	GOFLAGS='$(GOFLAGS)' go test ./...

integration-test:
	RUN_INTEGRATION=1 GOFLAGS='$(GOFLAGS)' go test -tags=integration ./cmd/pprof-mcp-server -run TestIntegrationAllTools -count=1

build-profctl:
	GOFLAGS='$(GOFLAGS)' go build -o bin/profctl ./cmd/profctl

build-server:
	GOFLAGS='$(GOFLAGS)' go build -o bin/pprof-mcp-server ./cmd/pprof-mcp-server

run-server:
	GOFLAGS='$(GOFLAGS)' go run ./cmd/pprof-mcp-server

install-profctl:
	GOFLAGS='$(GOFLAGS)' go install ./cmd/profctl

install-server:
	GOFLAGS='$(GOFLAGS)' go install ./cmd/pprof-mcp-server

install: install-profctl install-server

config:
	@echo "Example Claude Desktop config (update path as needed):"
	@printf '%s\n' '{' \
	  '  "mcpServers": {' \
	  '    "pprof-mcp": {' \
	  '      "command": "$(CURDIR)/bin/pprof-mcp-server",' \
	  '      "env": {' \
	  '        "DD_API_KEY": "your-api-key",' \
	  '        "DD_APP_KEY": "your-app-key"' \
	  '      }' \
	  '    }' \
	  '  }' \
	  '}'

clean:
	rm -rf bin
