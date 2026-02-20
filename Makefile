BINARY := awstbx
OUT_DIR := bin

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GOLANGCI_LINT_VERSION ?= v1.64.8
GORELEASER_VERSION ?= v2.13.3
GIT_CLIFF_VERSION ?= 2.12.0
GOBIN ?= $(shell go env GOPATH)/bin

GORELEASER_BIN ?= $(shell command -v goreleaser 2>/dev/null || echo $(GOBIN)/goreleaser)
GIT_CLIFF_BIN ?= $(shell command -v git-cliff 2>/dev/null || echo $(GOBIN)/git-cliff)

LDFLAGS := -X github.com/towardsthecloud/aws-toolbox/internal/version.Version=$(VERSION)
LDFLAGS += -X github.com/towardsthecloud/aws-toolbox/internal/version.Commit=$(COMMIT)
LDFLAGS += -X github.com/towardsthecloud/aws-toolbox/internal/version.Date=$(DATE)

.PHONY: help setup fmt lint test test-integration coverage build docs changelog changelog-current release-check release-snapshot tag

help: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-18s %s\n", $$1, $$2}'

setup: ## Install local development tooling
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)
	@set -eu; \
	if command -v git-cliff >/dev/null 2>&1 && git-cliff --version | grep -Eq " $(GIT_CLIFF_VERSION)([[:space:]]|$$)"; then \
		echo "git-cliff $(GIT_CLIFF_VERSION) already installed"; \
	else \
		os="$$(uname -s | tr '[:upper:]' '[:lower:]')"; \
		arch="$$(uname -m)"; \
		case "$$arch" in \
			x86_64) arch="x86_64" ;; \
			arm64|aarch64) arch="aarch64" ;; \
			*) echo "unsupported architecture: $$arch"; exit 1 ;; \
		esac; \
		case "$$os" in \
			linux) target="unknown-linux-gnu" ;; \
			darwin) target="apple-darwin" ;; \
			*) echo "unsupported os: $$os"; exit 1 ;; \
		esac; \
		archive="git-cliff-$(GIT_CLIFF_VERSION)-$$arch-$$target.tar.gz"; \
		url="https://github.com/orhun/git-cliff/releases/download/v$(GIT_CLIFF_VERSION)/$$archive"; \
		tmpdir="$$(mktemp -d)"; \
		curl -fsSL "$$url" -o "$$tmpdir/$$archive"; \
		tar -xzf "$$tmpdir/$$archive" -C "$$tmpdir"; \
		mkdir -p "$(GOBIN)"; \
		install "$$tmpdir/git-cliff-$(GIT_CLIFF_VERSION)/git-cliff" "$(GOBIN)/git-cliff"; \
		rm -rf "$$tmpdir"; \
	fi

fmt: ## Format Go code
	go fmt ./...

lint: ## Run golangci-lint
	golangci-lint run

test: ## Run unit tests
	go test -race ./...

test-integration: ## Run integration tests
	go test -tags=integration ./...

coverage: ## Enforce coverage gates
	go test -coverprofile=coverage.out ./internal/...
	@total_cov=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub("%","",$$3); print $$3}'); \
	echo "total coverage: $$total_cov%"; \
	awk -v cov="$$total_cov" 'BEGIN { if (cov+0 < 80) { print "coverage gate failed: must be >=80%"; exit 1 } }'

build: ## Build the awstbx binary
	mkdir -p $(OUT_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BINARY) ./cmd/awstbx

docs: ## Generate CLI markdown docs and man pages
	rm -rf docs/cli docs/man
	go run ./cmd/awstbx-docs

changelog: ## Generate CHANGELOG.md from git history
	$(GIT_CLIFF_BIN) --config cliff.toml --output CHANGELOG.md

changelog-current: ## Print current tag release notes with git-cliff
	$(GIT_CLIFF_BIN) --config cliff.toml --current --strip all

release-check: ## Validate GoReleaser configuration
	$(GORELEASER_BIN) check --config .goreleaser.yaml

release-snapshot: ## Build a local snapshot release without publishing
	$(GORELEASER_BIN) release --snapshot --clean --skip=publish --skip=announce --config .goreleaser.yaml

tag: ## Create and push annotated SemVer tag (VERSION=vX.Y.Z)
	@set -eu; \
	if [ "$(VERSION)" = "dev" ]; then \
		echo "VERSION is required. Example: make tag VERSION=v1.2.3"; \
		exit 1; \
	fi; \
	if ! printf '%s' "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$'; then \
		echo "VERSION must match vMAJOR.MINOR.PATCH"; \
		exit 1; \
	fi; \
	if [ -n "$$(git status --porcelain)" ]; then \
		echo "working tree must be clean before tagging"; \
		exit 1; \
	fi; \
	git fetch origin main --tags; \
	if ! git merge-base --is-ancestor HEAD origin/main; then \
		echo "HEAD must be reachable from origin/main"; \
		exit 1; \
	fi; \
	if git rev-parse "$(VERSION)" >/dev/null 2>&1; then \
		echo "tag $(VERSION) already exists locally"; \
		exit 1; \
	fi; \
	if git ls-remote --tags origin "refs/tags/$(VERSION)" | grep -q "$(VERSION)"; then \
		echo "tag $(VERSION) already exists on origin"; \
		exit 1; \
	fi; \
	git tag -a "$(VERSION)" -m "Release $(VERSION)"; \
	git push origin "$(VERSION)"; \
	echo "pushed tag $(VERSION)"
