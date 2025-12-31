SHELL := /bin/bash
GOFLAGS ?= -mod=vendor

.PHONY: all tidy vendor test build-profctl build-server run-server install-profctl config clean

all: vendor test build-profctl build-server

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

config:
	@cat <<'EOF'
{
  "mcpServers": {
    "pprof-mcp": {
      "command": "bash",
      "args": ["-lc", "cd /home/arreyder/repos/pprof-mcp && go run ./cmd/pprof-mcp-server"]
    }
  }
}
EOF

clean:
	rm -rf bin
