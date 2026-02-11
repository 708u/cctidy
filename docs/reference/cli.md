# CLI Reference

## Usage

```txt
cctidy [flags]
```

## Flags

| Flag                  | Short | Default | Description                       |
| --------------------- | ----- | ------- | --------------------------------- |
| `--target`            | `-t`  | (none)  | Path to a specific file to format |
| `--backup`            |       | false   | Create backup before writing      |
| `--dry-run`           |       | false   | Show changes without writing      |
| `--check`             |       | false   | Exit with 1 if any file is dirty  |
| `--include-bash-tool` |       | false   | Include Bash entries in sweeping  |
| `--verbose`           | `-v`  | false   | Show formatting details           |
| `--version`           |       |         | Print version                     |

## Target Files

When no `--target` is specified, cctidy processes 5 files
in order:

| File                            | Operations                |
| ------------------------------- | ------------------------- |
| `~/.claude.json`                | Path cleaning, formatting |
| `~/.claude/settings.json`       | Sweeping, sorting         |
| `~/.claude/settings.local.json` | Sweeping, sorting         |
| `.claude/settings.json`         | Sweeping, sorting         |
| `.claude/settings.local.json`   | Sweeping, sorting         |

Project-level settings files (`.claude/`) are resolved
relative to the current working directory.

### Single Target

With `--target FILE`, only the specified file is processed.

The formatter is chosen by filename:

- `.claude.json` applies path cleaning and formatting
- All other files apply sweeping and sorting

Missing files in single-target mode produce an error.
In default (multi-target) mode, missing files are skipped.

## Exit Codes

| Code | Meaning                             |
| ---- | ----------------------------------- |
| 0    | Success                             |
| 1    | `--check`: dirty files detected     |
| 2    | Invalid flags or runtime error      |

## Flag Constraints

`--check` cannot be combined with `--backup` or `--dry-run`.
Using them together exits with code 2.

## Backup

`--backup` creates a timestamped copy before writing:

```txt
{path}.backup.{YYYYMMDDhhmmss}
```

Example:

```txt
~/.claude.json.backup.20250210143022
```

The backup preserves the original file permissions.

## Dry Run

`--dry-run` runs all formatting and path cleaning logic
but skips writing files and creating backups.
Combined with `--verbose`, this shows what would change.

## Check Mode

`--check` compares the formatted output against the
current file contents without writing.

- Returns exit code 1 if any file needs formatting.
- Returns exit code 0 if all files are already formatted.
- With `--verbose`, lists each file that needs formatting.

## Verbose Output

### Single Target Output

```txt
Size: 1,234 -> 987 bytes
```

With backup:

```txt
Size: 1,234 -> 987 bytes
Backup: /path/to/.claude.json.backup.20250210143022
```

### Multiple Targets Output

```txt
/home/user/.claude.json:
  Projects: 5 -> 3 (removed 2)
  Size: 1,234 -> 987 bytes

/home/user/.claude/settings.json:
  (no changes)

/home/user/.claude/settings.local.json:
  skipped (not found)
```

## Atomic Write

File writes use a temp-file-then-rename strategy:

1. Write to a temporary file in the same directory
2. Sync to disk
3. Rename to the target path

This prevents partial writes on crash or interrupt.
Symlinks are resolved before writing so the actual
target file is updated.
