# Contribution Guidelines

Thank you for your interest in contributing to [AWS Toolbox](https://github.com/towardsthecloud/aws-toolbox).

## Reporting Bugs or Feature Requests

Use the GitHub issue tracker to report bugs or suggest features.
Before opening an issue, check existing open and recently closed issues first.

## Development Setup

This repository is now centered on the `awstbx` Go CLI.

1. Install Go (version from `go.mod`).
2. Install project tooling:
   ```bash
   make setup
   ```
3. Run local checks before opening a PR:
   ```bash
   make fmt
   make lint
   make test
   make build
   ```

## Pull Requests

Before opening a pull request:

1. Start from the latest `main` branch.
2. Keep the change focused and well-scoped.
3. Use clear commit messages (Conventional Commits preferred).
4. Ensure CI passes.

## Legacy Scripts

Historic Python/shell scripts are kept under `archived/` for reference only.
They are not the primary supported interface.

## Licensing

See the [LICENSE](https://github.com/towardsthecloud/aws-toolbox/blob/main/LICENSE) file for project licensing.
