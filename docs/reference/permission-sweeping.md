# Permission Sweeping

cctidy removes stale permission entries from settings
files. Only `permissions.allow` and `permissions.ask`
arrays are swept. `permissions.deny` entries are always
preserved because removing a stale deny rule could
silently re-enable a previously blocked action.

## Tool Coverage

### Swept

| Tool  | Tier   | Default  |
| ----- | ------ | -------- |
| Read  | safe   | enabled  |
| Edit  | safe   | enabled  |
| Bash  | unsafe | disabled |
| Task  | safe   | enabled  |
| Skill | safe   | enabled  |
| MCP   | safe   | enabled  |

Bash sweeping requires `--unsafe` flag or
`enabled = true` in the config file.
See [CLI Reference](cli.md#configuration-file)
for config details.

### Not Swept

| Tool      | Reason                              |
| --------- | ----------------------------------- |
| Write     | Target path is not yet created      |
| Grep      | No staleness criteria for patterns  |
| Glob      | Current match set may be transient  |
| WebFetch  | URL reachability is not reliable    |
| WebSearch | Query validity cannot be determined |

- Write: The target path does not exist yet because
  the tool creates new files. A missing path is the
  expected state, not a sign of staleness.
- Grep: The specifier is a regex pattern with no
  filesystem entity to validate against. There is
  no criterion to determine whether a pattern is
  stale.
- Glob: The specifier is a glob pattern. Even if
  no files currently match, the match set changes
  as files are added or removed. An empty result
  does not imply staleness.
- WebFetch: The specifier is a URL or domain. URL
  availability is transient due to network errors,
  authentication, or rate limiting. Checking
  reachability would produce false positives.
- WebSearch: The specifier is a search query. Any
  query string is always potentially valid and has
  no external state to check against.

### Not Yet Supported

| Tool         | Similar to |
| ------------ | ---------- |
| NotebookEdit | Edit       |

NotebookEdit uses a path-based specifier and could
be swept with the same logic as Read/Edit, but is
not yet implemented.

MultiEdit is not listed because its permission
entries are recorded as `Edit(...)` by Claude Code.

Entries for any other unrecognized tool are kept
as-is.

## Safety Tiers

Sweepers are classified as safe or unsafe:

- **Safe**: Run unconditionally on every invocation
- **Unsafe**: Require `--unsafe` flag or config opt-in

Bash sweeping is the only unsafe sweeper. It uses
path extraction heuristics that may produce false
positives.

Config `[permission.bash] enabled = true` promotes Bash
to safe tier (always active without `--unsafe`).
See [CLI Reference](cli.md#configuration-file).

## Read / Edit

Each entry has the form `Tool(specifier)`.
The specifier is resolved to an absolute path and checked
for existence.

### Path Resolution

| Prefix    | Resolution                   | Requires   |
| --------- | ---------------------------- | ---------- |
| `//path`  | Strip leading `/` -> `/path` | (none)     |
| `~/path`  | Join with home directory     | homeDir    |
| `/path`   | Join with base directory     | projectDir |
| `./path`  | Join with base directory     | projectDir |
| `../path` | Join with base directory     | projectDir |

- `homeDir` is the user's home directory
- `projectDir` is the project root for project-level
  settings, or empty for global settings

### Skipped Entries

The following entries are always kept:

- Contains glob characters (`*`, `?`, `[`)
- Required directory (homeDir or projectDir) is not set
- Path exists on the filesystem

## Bash

Enabled with `--unsafe` flag or
`[permission.bash] enabled = true` in config.

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

| Prefix    | Resolution               | Requires   |
| --------- | ------------------------ | ---------- |
| `/path`   | Used as-is (absolute)    | (none)     |
| `~/path`  | Join with home directory | homeDir    |
| `./path`  | Join with base directory | projectDir |
| `../path` | Join with base directory | projectDir |

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

| Entry                                    | Result | Reason               |
| ---------------------------------------- | ------ | -------------------- |
| `Bash(git -C /dead/repo status)`         | swept  | all paths dead       |
| `Bash(cp /alive/src /dead/dst)`          | kept   | `/alive/src` exists  |
| `Bash(npm run *)`                        | kept   | no extractable paths |
| `Bash(cat ./dead/file)` (no projectDir)  | kept   | path unresolvable    |

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

### Task Always Kept

The following entries are never swept:

- **Built-in agents**: `Bash`, `Explore`, `Plan`,
  `claude-code-guide`, `general-purpose`,
  `statusline-setup`
- **Plugin agents**: specifier contains `:` (e.g.
  `plugin:my-agent`). These are handled by the
  [plugin sweeper](#plugin) instead.
- **No context**: when neither `homeDir` nor `projectDir`
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

### Task Sweep Logic

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

| Entry (project settings)                | Result | Reason            |
| --------------------------------------- | ------ | ----------------- |
| `Task(Explore)`                         | kept   | built-in agent    |
| `Task(plugin:my-agent)`                 | kept   | plugin agent      |
| `Task(custom-name)` (frontmatter)       | kept   | frontmatter match |
| `Task(home-agent)` (.md in home only)   | swept  | not in project    |
| `Task(dead-agent)`                      | swept  | agent not found   |

## Skill

Always active.

Skill entries have the form `Skill(name)` or
`Skill(name *)`. The sweeper checks whether the
referenced skill or command still exists.

Claude Code merges custom slash commands into skills.
Both `.claude/skills/<name>/SKILL.md` and
`.claude/commands/<name>.md` create the same `/name`
command. The sweeper checks both directories.

### Skill Always Kept

The following entries are never swept:

- **Plugin skills**: specifier contains `:` (e.g.
  `plugin:skill-name`). These are handled by the
  [plugin sweeper](#plugin) instead.
- **No context**: when neither `homeDir` nor `projectDir`
  is available, entries are kept conservatively

### Skill Name Resolution

Skill names are resolved from two sources under the
`.claude/` directory:

| Source | Path | Name |
| ------ | ---- | ---- |
| Skills | `skills/<dir>/SKILL.md` | frontmatter `name`, else dir name |
| Commands | `commands/<file>.md` | frontmatter `name`, else filename |

For skills, a subdirectory must contain `SKILL.md`
to be recognized. If SKILL.md has a YAML frontmatter
`name` field, that value is used as the skill name;
otherwise the subdirectory name is used.

For commands, if the `.md` file has a YAML
frontmatter `name` field, that value is used;
otherwise the filename without extension is used.

### Sweep Logic

Skill lookup is scoped to the settings level:

- **Project-level settings** (`.claude/settings.json`
  in project): scans only
  `<project>/.claude/skills/` and
  `<project>/.claude/commands/`
- **User-level settings** (`~/.claude/settings.json`):
  scans only `~/.claude/skills/` and
  `~/.claude/commands/`

For entries with a space (e.g. `Skill(name *)`),
only the first token before the space is used as
the skill name.

An entry is swept when:

1. The specifier does not contain `:` (not a plugin)
2. The skill name does not appear in the resolved set

### Skill Examples

| Entry (project settings) | Result | Reason |
| --- | --- | --- |
| `Skill(plugin:name)` | kept | plugin skill |
| `Skill(review)` (SKILL.md) | kept | skill exists |
| `Skill(deploy)` (.md cmd) | kept | command exists |
| `Skill(review *)` | kept | name extracted |
| `Skill(dead-skill)` | swept | not found |

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

Missing files are silently ignored.

### Scope Separation

Claude Code saves MCP tool permissions to different
settings files based on the server's definition scope.
cctidy mirrors this by using a different known set per
settings file:

| Settings file scope  | Known server set               |
| -------------------- | ------------------------------ |
| User (`~/.claude/`)  | `~/.claude.json` only          |
| Project (`.claude/`) | `.mcp.json` + `~/.claude.json` |

User settings only contain entries for servers defined
in `~/.claude.json`. Project settings can contain
entries from both `.mcp.json` and `~/.claude.json`.

### MCP Sweep Logic

An entry is swept when **both** of these are true:

1. The tool name starts with `mcp__` (but not
   `mcp__plugin_`)
2. The extracted server name is not in the known set
   for the target file's scope

Marketplace plugin entries (`mcp__plugin_*`) bypass the
MCP sweeper and are handled by the plugin sweeper
instead. See the [Plugin](#plugin) section.

### Bare Entries

Both forms are supported:

- `mcp__slack__post_message` (with tool name)
- `mcp__slack` (bare server reference)

## Plugin

Always active. Safe tier.

Plugin entries are permission entries associated with
a marketplace plugin. The sweeper checks whether the
plugin is still enabled in `enabledPlugins`.

### Plugin Entry Formats

Three tool types can have plugin entries:

| Type  | Format | Plugin name |
| ----- | ------ | ----------- |
| MCP   | `mcp__plugin_<name>_<server>__<tool>` | first `_` token after `mcp__plugin_` |
| Skill | `Skill(<name>:<suffix>)` | part before `:` |
| Task  | `Task(<name>:<suffix>)` | part before `:` |

### enabledPlugins Discovery

The `enabledPlugins` map is collected from all settings
files:

- `~/.claude/settings.json`
- `~/.claude/settings.local.json`
- `.claude/settings.json`
- `.claude/settings.local.json`

Keys have the form `name@marketplace`. The plugin name
is the part before `@`. When the same plugin name
appears in multiple keys (different marketplaces),
the values are OR-merged: if any is `true`, the plugin
is considered enabled.

### Plugin Sweep Logic

A plugin entry is swept when **all** of these are true:

1. The entry is identified as a plugin entry
2. `enabledPlugins` exists in at least one settings
   file
3. The plugin name is explicitly registered as
   disabled (`false`) in the merged map

The following entries are always kept:

- When no file contains `enabledPlugins` (sweeper
  inactive)
- When the plugin name is not present in the merged
  map (unknown plugin, conservative)
- When the plugin is enabled (`true`)
- `permissions.deny` entries (never swept)

### Plugin Examples

Given `enabledPlugins`:

```json
{
  "github@claude-plugins-official": true,
  "linter@acme-tools": false
}
```

| Entry | Result | Reason |
| --- | --- | --- |
| `mcp__plugin_github_github__search_code` | kept | github enabled |
| `mcp__plugin_linter_acme__check` | swept | linter disabled |
| `Skill(github:review)` | kept | github enabled |
| `Skill(linter:lint-check)` | swept | linter disabled |
| `Task(linter:lint-agent)` | swept | linter disabled |
| `Skill(plugin:my-skill)` | kept | unknown plugin |
| `mcp__slack__post_message` | (MCP) | handled by MCP sweeper |
