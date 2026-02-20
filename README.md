# awstbx

`awstbx` is the Go CLI replacement for the legacy AWS Python/Bash scripts in this repository.

It provides:
- One consistent command surface: `awstbx <service> <action>`
- Shared auth + region/profile handling
- Safe defaults (`--dry-run`, confirmation prompts)
- Structured output (`table`, `json`, `text`)

## Installation

### Homebrew

```bash
brew tap towardsthecloud/tap
brew install awstbx
```

### Build from source

```bash
git clone https://github.com/towardsthecloud/aws-toolbox.git
cd aws-toolbox
make build
./bin/awstbx --help
```

## Quick Start

```bash
# Inspect available commands
awstbx --help

# Use a profile/region override
awstbx --profile platform --region us-east-1 ec2 list-eips

# Preview destructive actions
awstbx s3 delete-buckets --empty --dry-run

# Machine-readable output
awstbx cloudwatch count-log-groups --output json
```

## Global Flags

| Flag | Description |
| --- | --- |
| `--profile`, `-p` | AWS CLI profile name |
| `--region`, `-r` | AWS region override |
| `--dry-run` | Preview changes without executing |
| `--output`, `-o` | Output format: `table`, `json`, `text` |
| `--no-confirm` | Skip interactive confirmation prompts |
| `--version` | Print build metadata |

## Command Groups

`awstbx` currently includes:

- `appstream`
- `cfn`
- `cloudwatch`
- `ec2`
- `ecs`
- `efs`
- `iam`
- `kms`
- `org`
- `r53`
- `s3`
- `sagemaker`
- `ssm`
- `completion`
- `version`

Use `awstbx <group> --help` and `awstbx <group> <command> --help` for command-level usage and examples.

## Shell Completions

Generate shell completion scripts with:

```bash
awstbx completion [bash|zsh|fish|powershell]
```

### Bash

```bash
mkdir -p ~/.local/share/bash-completion/completions
awstbx completion bash > ~/.local/share/bash-completion/completions/awstbx
```

### Zsh

```bash
mkdir -p ~/.zfunc
awstbx completion zsh > ~/.zfunc/_awstbx
# Ensure ~/.zfunc is in your fpath
```

### Fish

```bash
mkdir -p ~/.config/fish/completions
awstbx completion fish > ~/.config/fish/completions/awstbx.fish
```

### PowerShell

```powershell
awstbx completion powershell > $PROFILE.CurrentUserAllHosts
```

## CLI Reference and Man Pages

Auto-generated command docs:

- Markdown reference: `docs/cli/`
- Man pages: `docs/man/`

Regenerate docs after command/help changes:

```bash
make docs
```

## Migration Guide

Use `docs/migration.md` for the complete mapping from legacy scripts to `awstbx` commands.

## Local Development

```bash
make setup
make fmt
make lint
make test
make test-integration
make coverage
make build
make docs
```

## Legacy Scripts

Legacy scripts are kept under `old-scripts/` during migration, but `awstbx` is the primary supported interface.
