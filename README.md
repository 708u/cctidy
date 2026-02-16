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
> Bash tool sweeping is opt-in via `--unsafe` flag
> or `[sweep.bash] enabled = true` in config. Bash
> entries use path extraction heuristics that may
> produce false positives, so sweeping is disabled
> by default.

### Supported Tools

| Tool  | Default  | Detection               |
| ----- | -------- | ----------------------- |
| Read  | enabled  | Path existence          |
| Edit  | enabled  | Path existence          |
| Bash  | disabled | All extracted paths     |
| Task  | enabled  | Agent existence         |
| Skill | enabled  | Skill/command existence |
| MCP   | enabled  | Server registration     |

Entries for tools not listed above (e.g. `Write`,
`Grep`, `WebFetch`) are kept unchanged. Write is
excluded because it creates new files, so the target
path not existing is expected.

## Installation

### Homebrew

```bash
brew install 708u/tap/cctidy
```

### Install script

Linux / macOS:

```bash
curl -sSfL https://raw.githubusercontent.com/708u/cctidy/main/scripts/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/708u/cctidy/main/scripts/install.ps1 | iex
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

# Include unsafe sweepers (e.g. Bash)
cctidy --unsafe
```

## CLI Options

| Flag                  | Short | Description                       |
| --------------------- | ----- | --------------------------------- |
| `--target`            | `-t`  | Format a specific file only       |
| `--backup`            |       | Create backup before writing      |
| `--dry-run`           |       | Show changes without writing      |
| `--check`             |       | Exit with 1 if any file is dirty  |
| `--unsafe`            |       | Enable unsafe sweepers (e.g. Bash)|
| `--config`            |       | Path to config file               |
| `--verbose`           | `-v`  | Show formatting details           |
| `--version`           |       | Print version                     |

Details:
[docs/reference/cli.md](docs/reference/cli.md)

## Exit Codes

| Code | Meaning                           |
| ---- | --------------------------------- |
| 0    | Success                           |
| 1    | `--check`: dirty files detected   |
| 2    | Invalid flags or runtime error    |

`--check` cannot be combined with `--backup` or
`--dry-run`. Using them together exits with code 2.

## Configuration

cctidy supports layered TOML configuration. Settings
are merged in the following order (later wins):

1. Global: `~/.config/cctidy/config.toml`
2. Project shared: `.claude/cctidy.toml`
3. Project local: `.claude/cctidy.local.toml`
4. CLI flags (`--unsafe`)

Project config files are searched from the nearest
`.claude/` directory, walking up from the current
working directory.

### Example

```toml
# ~/.config/cctidy/config.toml or .claude/cctidy.toml
[sweep.bash]
enabled = true
exclude_entries = ["mkdir -p /opt/logs"]
exclude_commands = ["mkdir", "touch"]
exclude_paths = ["vendor/"]
```

### Merge Strategy

- **Scalars** (`enabled`): last-set-wins. Unset values
  do not override lower layers.
- **Arrays** (`exclude_*`): union with deduplication.
  Each layer adds entries additively.
- **Relative paths** in project config `exclude_paths`
  are resolved against the project root.

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

## File Safety

- **Atomic write**: Uses temp file + rename to
  prevent partial writes on crash or interrupt.
- **Symlink**: Resolves symlinks before writing so
  the actual target file is updated.

## Claude Code Plugin

A Claude Code plugin is available that automatically
formats config files on session start.

```txt
/plugin marketplace add 708u/cctidy
/plugin install cctidy@708u-cctidy
```

## License

[MIT](LICENSE)
