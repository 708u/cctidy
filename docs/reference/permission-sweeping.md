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
| Bash  | disabled |
| Task  | enabled  |
| MCP   | enabled  |

Bash sweeping requires `--sweep-bash` flag or
`enabled = true` in the config file.
See [CLI Reference](cli.md#configuration-file)
for config details.

Entries for tools not listed above (e.g. `Write`,
`WebFetch`, `Grep`) are kept unchanged. Write entries
are excluded because the tool creates new files, so
the target path not existing is expected.

## Read / Edit

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

Enabled with `--sweep-bash`.

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

### Bash Sweep Logic

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

### Exclude Patterns

When a config file is provided, Bash entries matching
exclude patterns are always kept (never swept).

Three exclusion types are available:

| Type               | Match method        | Example               |
| ------------------ | ------------------- | --------------------- |
| `exclude_entries`  | Exact specifier     | `mkdir -p /opt/logs`  |
| `exclude_commands` | First token (space) | `mkdir`, `touch`      |
| `exclude_paths`    | Path prefix         | `/opt/myapp/`         |

Checks are applied in order: entries, commands, paths.
The first match wins.

For `exclude_paths`, trailing `/` is recommended to
ensure directory boundary matching.

## Task

Always active.

Task entries have the form `Task(AgentName)`.
The sweeper checks whether the referenced agent still
exists.

### Always Kept

The following entries are never swept:

- **Built-in agents**: `Bash`, `Explore`, `Plan`,
  `claude-code-guide`, `general-purpose`,
  `statusline-setup`
- **Plugin agents**: specifier contains `:` (e.g.
  `plugin:my-agent`)
- **No context**: when neither `homeDir` nor `baseDir`
  is available, entries are kept conservatively

### Agent Name Resolution

Agent names are resolved exclusively from the YAML
frontmatter `name` field in `.md` files in the agents
directory. Filenames are not used for identification.

```markdown
---
name: custom-name
---
```

Files without a valid `name` field are skipped.

### Sweep Logic

Agent lookup is scoped to the settings level:

- **Project-level settings** (`.claude/settings.json`
  in project): scans only
  `<project>/.claude/agents/`
- **User-level settings** (`~/.claude/settings.json`):
  scans only `~/.claude/agents/`

An entry is swept when:

1. The agent is not built-in
2. The specifier does not contain `:` (not a plugin)
3. The agent name does not appear in the resolved set

### Task Examples

| Entry (project settings)               | Result | Reason            |
| -------------------------------------- | ------ | ----------------- |
| `Task(Explore)`                        | kept   | built-in agent    |
| `Task(plugin:my-agent)`               | kept   | plugin agent      |
| `Task(custom-name)` (frontmatter)      | kept   | frontmatter match |
| `Task(home-agent)` (.md in home only)  | swept  | not in project    |
| `Task(dead-agent)`                     | swept  | agent not found   |

## MCP

Always active.

MCP entries use the `mcp__<server>__<tool>` naming
convention. The sweeper checks whether the server is
still registered in `.mcp.json` or `~/.claude.json`.

### Server Discovery

Known servers are collected from two sources:

| Source           | Key path                             |
| ---------------- | ------------------------------------ |
| `.mcp.json`      | `mcpServers.<name>`                  |
| `~/.claude.json` | `mcpServers.<name>`                  |
| `~/.claude.json` | `projects.<path>.mcpServers.<name>`  |

The union of all discovered server names forms the
known set. Missing files are silently ignored.

### MCP Sweep Logic

An entry is swept when **both** of these are true:

1. The tool name starts with `mcp__` (but not
   `mcp__plugin_`)
2. The extracted server name is not in the known set

Marketplace plugin entries (`mcp__plugin_*`) are always
kept because they are managed by the plugin system, not
by `.mcp.json`.

### Bare Entries

Both forms are supported:

- `mcp__slack__post_message` (with tool name)
- `mcp__slack` (bare server reference)
