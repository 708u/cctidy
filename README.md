# cctidy

A CLI tool that formats Claude Code configuration files.

## Motivation

Claude Code generates and updates several JSON config
files during use. Over time, these files accumulate
stale entries (removed projects, dead permission paths)
and inconsistent formatting. cctidy keeps them tidy by
removing dead references and normalizing structure,
so diffs stay clean and configs stay readable.

## Features

cctidy handles three categories of config files, each
with different operations:

### ~/.claude.json

Removes project paths and GitHub repo paths that no
longer exist on the filesystem. Repo keys with no
remaining paths are deleted entirely. Pretty-prints
with sorted keys.

> [!NOTE]
> Arrays are not sorted because Claude Code manages
> their order internally.

### Global settings (~/.claude/settings\*.json)

Removes `allow` and `ask` permission entries that
reference non-existent paths. `deny` entries are never
swept to preserve safety. Pretty-prints with sorted
keys and sorted homogeneous arrays for deterministic
diffs.

### Project settings (.claude/settings\*.json)

Same operations as global settings, with the addition
of project-relative path resolution for permission
sweeping.

> [!NOTE]
> Bash tool sweeping is opt-in via `--include-bash-tool`.
> Bash entries may contain paths that do not yet exist
> (e.g. output paths for `mkdir`, `touch`), so automatic
> sweeping could remove intentional permissions.

## Installation

### Homebrew

```bash
brew install 708u/tap/cctidy
```

### Go

```bash
go install github.com/708u/cctidy/cmd/cctidy@latest
```

Or download from
[Releases](https://github.com/708u/cctidy/releases).

## Quick Start

```bash
# Format all target files
cctidy

# Preview changes without writing
cctidy --dry-run -v

# Exit with 1 if any file needs formatting.
# Useful for CI to enforce consistent config formatting.
cctidy --check

# Format a specific file with backup
cctidy -t ~/.claude.json --backup

# Include Bash tool sweeping
cctidy --include-bash-tool
```

## CLI Options

| Flag                  | Short | Description                       |
| --------------------- | ----- | --------------------------------- |
| `--target`            | `-t`  | Format a specific file only       |
| `--backup`            |       | Create backup before writing      |
| `--dry-run`           |       | Show changes without writing      |
| `--check`             |       | Exit with 1 if any file is dirty  |
| `--include-bash-tool` |       | Include Bash entries in sweeping  |
| `--verbose`           | `-v`  | Show formatting details           |
| `--version`           |       | Print version                     |

Details:
[docs/reference/cli.md](docs/reference/cli.md)

## Target Files

| File                            | Operations                 |
| ------------------------------- | -------------------------- |
| `~/.claude.json`                | Path cleaning, formatting  |
| `~/.claude/settings.json`       | Sweeping, sorting          |
| `~/.claude/settings.local.json` | Sweeping, sorting          |
| `.claude/settings.json`         | Sweeping, sorting          |
| `.claude/settings.local.json`   | Sweeping, sorting          |

Details:
[docs/reference/formatting.md](docs/reference/formatting.md),
[docs/reference/permission-sweeping.md](docs/reference/permission-sweeping.md)

## Claude Code Plugin

A Claude Code plugin is available that automatically
formats config files on session start.

```txt
/plugin marketplace add 708u/cctidy
/plugin install cctidy@708u-cctidy
```

## License

[MIT](LICENSE)
