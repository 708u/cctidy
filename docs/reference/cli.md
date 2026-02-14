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
| `--sweep-bash`        |       | false   | Include Bash entries in sweeping  |
| `--config`            |       | (auto)  | Path to config file               |
| `--verbose`           | `-v`  | false   | Show formatting details           |
| `--version`           |       |         | Print version                     |

## Configuration File

cctidy reads TOML configuration files for settings
that are too verbose for CLI flags (e.g. exclude
patterns). Settings are merged in layers, with later
layers overriding earlier ones.

### Config Layers

| Priority | File                            | Scope   |
| -------- | ------------------------------- | ------- |
| 1 (low)  | `~/.config/cctidy/config.toml`  | Global  |
| 2        | `.claude/cctidy.toml`           | Project |
| 3        | `.claude/cctidy.local.toml`     | Local   |
| 4 (high) | CLI flags (`--sweep-bash`)      | Runtime |

The global config path can be overridden with
`--config PATH`.

Project config files are located in the `.claude/`
directory of the project root. The project root is
found by walking up from the current working directory
to the first directory containing a `.claude/` folder.

If no config files are found, cctidy uses defaults.

### Merge Strategy

- **Scalars** (`enabled`): last-set-wins. Unset values
  do not override lower layers.
- **Arrays** (`exclude_*`): union with deduplication.
  Each layer adds entries; no layer can remove entries
  from a lower layer.
- **Relative paths** in project config `exclude_paths`
  are resolved against the project root.

### Example

Global config (`~/.config/cctidy/config.toml`):

```toml
[sweep.bash]
exclude_commands = ["mkdir", "touch"]
```

Project shared config (`.claude/cctidy.toml`):

```toml
[sweep.bash]
enabled = true
exclude_paths = ["vendor/"]
```

Project local config (`.claude/cctidy.local.toml`):

```toml
[sweep.bash]
exclude_commands = ["ln"]
```

Merged result: `enabled = true`,
`exclude_commands = ["mkdir", "touch", "ln"]`,
`exclude_paths = ["<projectRoot>/vendor/"]`.

### Config Fields

#### `[sweep.bash]`

| Key                | Type     | Default | Description                |
| ------------------ | -------- | ------- | -------------------------- |
| `enabled`          | bool     | (unset) | Enable Bash sweep          |
| `exclude_entries`  | string[] | []      | Specifiers to keep (exact) |
| `exclude_commands` | string[] | []      | Commands to keep (first    |
|                    |          |         | token match)               |
| `exclude_paths`    | string[] | []      | Path prefixes to keep      |

### Priority: CLI vs Config

| config `enabled` | `--sweep-bash` | Result    |
| ---------------- | --------------------- | --------- |
| unset / false    | absent                | sweep OFF |
| unset / false    | present               | sweep ON  |
| true             | absent                | sweep ON  |
| true             | present               | sweep ON  |

The CLI flag always wins. Exclude patterns are
config-only (no CLI flags).

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
relative to the project root. The project root is found
by walking up from the current working directory to the
first directory containing a `.claude/` folder. If none
is found, the current working directory is used.

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
