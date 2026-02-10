# Array sorting in ~/.claude.json

cctidy does **not** sort arrays in `~/.claude.json`.

Array element order is preserved as-is from the original
file. This means fields such as `allowedTools`,
`enabledMcpjsonServers`, and `githubRepoPaths` values
retain the order written by Claude Code.

Key sorting still occurs because Go's `encoding/json`
encoder sorts map keys alphabetically on marshal.

## Settings files

For settings files (`settings.json`, `settings.local.json`),
homogeneous arrays (string, number, bool) **are** sorted.
This is intentional because settings files benefit from
deterministic ordering for diffability.
