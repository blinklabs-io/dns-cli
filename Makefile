.PHONY: build test vet lint coverage clean

BINARY ?= dns-cli
VERSION ?= dev
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
# Prefer sibling dns-contracts HEAD; else last commit that touched demo fixtures.
CONTRACT_REV ?= $(shell \
	if [ -d ../dns-contracts/.git ] || [ -d ../dns-contracts ]; then \
		git -C ../dns-contracts rev-parse --short HEAD 2>/dev/null; \
	else \
		git log -1 --format=%h -- demo/fixtures/contracts 2>/dev/null; \
	fi || echo unknown)
PKG := github.com/blinklabs-io/dns-cli/internal/cli
LDFLAGS := -X $(PKG).Version=$(VERSION) \
	-X $(PKG).GitCommit=$(GIT_COMMIT) \
	-X $(PKG).BuildDate=$(BUILD_DATE) \
	-X $(PKG).ContractRevision=$(CONTRACT_REV)

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
