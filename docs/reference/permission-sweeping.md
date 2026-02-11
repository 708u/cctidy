# Permission Sweeping

cctidy removes stale permission entries from settings
files. Only `permissions.allow` and `permissions.ask`
arrays are swept. `permissions.deny` entries are always
preserved because removing a stale deny rule could
silently re-enable a previously blocked action.

## Supported Tools

| Tool  | Default  |
| ----- | -------- |
| Read  | enabled  |
| Edit  | enabled  |
| Write | enabled  |
| Bash  | disabled |

Bash sweeping requires the `--include-bash-tool` flag.

Entries for tools not listed above (e.g. `WebFetch`,
`Grep`) are kept unchanged.

## Read / Edit / Write

Each entry has the form `Tool(specifier)`.
The specifier is resolved to an absolute path and checked
for existence.

### Path Resolution

| Prefix    | Resolution                   | Requires |
| --------- | ---------------------------- | -------- |
| `//path`  | Strip leading `/` -> `/path` | (none)   |
| `~/path`  | Join with home directory     | homeDir  |
| `/path`   | Join with base directory     | baseDir  |
| `./path`  | Join with base directory     | baseDir  |
| `../path` | Join with base directory     | baseDir  |

- `homeDir` is the user's home directory
- `baseDir` is the project root for project-level
  settings, or empty for global settings

### Skipped Entries

The following entries are always kept:

- Contains glob characters (`*`, `?`, `[`)
- Required directory (homeDir or baseDir) is not set
- Path exists on the filesystem

## Bash

Enabled with `--include-bash-tool`.

Bash entries have the form `Bash(command string)`.
The sweeper extracts all paths from the command and
checks whether they exist.

### Path Extraction

**Absolute paths**: Extracted by regex matching
`/[A-Za-z0-9_./-]+`. Stops at glob characters,
shell metacharacters, whitespace, parentheses,
`$`, and braces.

**Relative paths**: Matches `./path`, `../path`,
and `~/path` prefixes. Bare relative paths
(e.g. `src/file`) are excluded to avoid false positives.

### Resolution

| Prefix    | Resolution               | Requires |
| --------- | ------------------------ | -------- |
| `/path`   | Used as-is (absolute)    | (none)   |
| `~/path`  | Join with home directory | homeDir  |
| `./path`  | Join with base directory | baseDir  |
| `../path` | Join with base directory | baseDir  |

Paths whose required directory is not set are
excluded from evaluation (treated as unresolvable).

### Sweep Logic

An entry is swept only when **all** of these are true:

1. At least one path was extracted from the command
2. Every extracted path is non-existent

If no paths can be extracted (e.g. `Bash(npm run *)`),
the entry is kept.

If at least one path exists
(e.g. `Bash(cp /alive/src /dead/dst)`),
the entry is kept.

### Examples

| Entry                                 | Result | Reason               |
| ------------------------------------- | ------ | -------------------- |
| `Bash(git -C /dead/repo status)`      | swept  | all paths dead       |
| `Bash(cp /alive/src /dead/dst)`       | kept   | `/alive/src` exists  |
| `Bash(npm run *)`                     | kept   | no extractable paths |
| `Bash(cat ./dead/file)` (no baseDir)  | kept   | path unresolvable    |
