# Formatting Rules

cctidy applies two formatters depending on the target file.

## Common

Both formatters:

- Pretty-print with 2-space indentation
- Sort object keys alphabetically
  (implicit via Go's `encoding/json` encoder)

## ~/.claude.json

Path cleaning is applied in addition to pretty-printing.

### Projects Cleaning

The `projects` key maps project directory paths to
metadata objects. cctidy removes entries whose paths
no longer exist on the filesystem.

If the `projects` key is missing, an empty object is
created.

### GitHub Repo Paths Cleaning

The `githubRepoPaths` key maps repository names
(e.g. `owner/repo`) to arrays of local checkout paths.
cctidy:

- Removes individual paths that no longer exist
- Deletes the entire repo key when all its paths are gone

If the `githubRepoPaths` key is missing, an empty object
is created.

### Array Ordering

Arrays in `~/.claude.json` are **not** sorted.
Element order is preserved as written by Claude Code.
This applies to fields such as `allowedTools`,
`enabledMcpjsonServers`, and `githubRepoPaths` values.

## Settings Files

Array sorting and permission sweeping are applied.

Target files:

- `~/.claude/settings.json`
- `~/.claude/settings.local.json`
- `.claude/settings.json`
- `.claude/settings.local.json`

### Array Sorting

Homogeneous arrays (all elements are the same primitive
type) are sorted:

| Element Type | Sort Order            |
| ------------ | --------------------- |
| string       | Lexicographic         |
| number       | Numeric               |
| bool         | `false` before `true` |

Mixed-type arrays and arrays of objects are left as-is.

### Permission Sweeping

Permission entries in `permissions.allow` and
`permissions.ask` that reference non-existent paths
are removed.

`permissions.deny` entries are never swept.

Details:
[permission-sweeping.md](permission-sweeping.md)
