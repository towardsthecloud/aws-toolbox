# Repository Guidelines

## Project Structure & Module Organization
- `cmd/awstbx/`: CLI entrypoint.
- `cmd/awstbx-docs/`: doc generator used by `make docs`.
- `internal/`: application code. Key areas include `internal/cli` (root command wiring), `internal/service/<aws-group>` (service commands), and shared packages like `internal/aws`, `internal/output`, and `internal/cliutil`.
- `docs/cli/` and `docs/man/`: generated command docs and man pages.
- `archived/`: legacy scripts kept for reference during migration.
- `icons/`: repository/branding assets.

## Build, Test, and Development Commands
- `make setup`: install local toolchain (`golangci-lint`, `goreleaser`, `git-cliff`).
- `make fmt`: run `go fmt ./...`.
- `make lint`: run `golangci-lint run`.
- `make test`: run unit tests with race detector.
- `make test-integration`: run integration-tagged tests.
- `make coverage`: enforce coverage gate for `internal/...` (must be `>=80%`).
- `make build`: compile `bin/awstbx` with version metadata.
- `make docs`: regenerate CLI markdown + man pages.

## Coding Style & Naming Conventions
- Follow standard Go formatting (`go fmt`) and keep lint clean (`golangci-lint`).
- Keep package names lowercase and focused by domain (`internal/service/s3`, `internal/service/ec2`, etc.).
- CLI behavior should follow the established command shape: `awstbx <service> <action>`.
- When command behavior or flags change, update help text and generated docs (`make docs`).

## Testing Guidelines
- Use Go’s `testing` package with colocated `*_test.go` files.
- Prefer table-driven tests for command and AWS-client behavior.
- Run `make test` locally before opening a PR; run `make coverage` for regression-risky changes.
- User-visible behavior changes should include or update tests.

## Commit & Pull Request Guidelines
- Use Conventional Commits (`feat:`, `fix:`, `refactor:`, `build:`, `ci:`, `test:`, `style:`, `docs:`, `chore:`).
- Keep PRs focused and rebased on the latest `main`.
- PR checklist minimum: run `make lint`, `make test`, `make build`; document doc/help updates and test coverage for behavior changes.
- Include validation details in the PR body (OS tested, `awstbx --version`, commands exercised).
