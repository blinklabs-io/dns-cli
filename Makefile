.PHONY: build test vet lint coverage clean

BINARY ?= dns-cli
VERSION ?= dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/blinklabs-io/dns-cli/internal/cli.Version=$(VERSION) \
	-X github.com/blinklabs-io/dns-cli/internal/cli.GitCommit=$(GIT_COMMIT) \
	-X github.com/blinklabs-io/dns-cli/internal/cli.BuildDate=$(BUILD_DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/dns-cli

test:
	go test ./...

vet:
	go vet ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out
