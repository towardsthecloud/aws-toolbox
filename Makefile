BINARY := awstbx
OUT_DIR := bin

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/towardsthecloud/aws-toolbox/internal/version.Version=$(VERSION)
LDFLAGS += -X github.com/towardsthecloud/aws-toolbox/internal/version.Commit=$(COMMIT)
LDFLAGS += -X github.com/towardsthecloud/aws-toolbox/internal/version.Date=$(DATE)

.PHONY: help setup fmt lint test test-integration coverage build docs

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

setup: ## Install local development tooling
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8

fmt: ## Format Go code
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run

test: ## Run unit tests
	go test -race ./...

test-integration: ## Run integration tests
	go test -tags=integration ./...

coverage: ## Enforce coverage gates
	go test -coverprofile=coverage.cli.out ./internal/cli
	@cli_cov=$$(go tool cover -func=coverage.cli.out | awk '/^total:/ {gsub("%","",$$3); print $$3}'); \
	echo "internal/cli coverage: $$cli_cov%"; \
	awk -v cov="$$cli_cov" 'BEGIN { if (cov+0 < 80) { print "coverage gate failed: internal/cli must be >=80%"; exit 1 } }'

build: ## Build the awstbx binary
	mkdir -p $(OUT_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY) ./cmd/awstbx

docs: ## Generate CLI markdown docs and man pages
	rm -rf docs/cli docs/man
	go run ./cmd/awstbx-docs
