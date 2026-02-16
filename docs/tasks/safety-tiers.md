# Safety Tiers

## Overview

cctidy classifies permission sweepers into two safety
tiers. Safe sweepers run unconditionally. Unsafe
sweepers require explicit opt-in.

## Safety Classification

| Tool  | Tier   | Rationale                     |
| ----- | ------ | ----------------------------- |
| Read  | safe   | Path existence check only     |
| Edit  | safe   | Path existence check only     |
| Task  | safe   | Agent existence check only    |
| Skill | safe   | Skill/command existence check |
| MCP   | safe   | Server existence check only   |
| Bash  | unsafe | Command string heuristics     |

Bash sweeping relies on path extraction heuristics
from command strings. False positives can remove
intentional entries, so it defaults to opt-in.

## Enabling Unsafe Sweepers

Two mechanisms:

1. **CLI flag**: `--unsafe` enables all unsafe sweepers
2. **Config file**: `[sweep.bash] enabled = true`
   promotes Bash to safe tier (always active)

### Priority

| config `enabled` | `--unsafe` | Bash sweep | Tier   |
| ---------------- | ---------- | ---------- | ------ |
| unset / false    | absent     | OFF        | -      |
| unset / false    | present    | ON         | unsafe |
| true             | absent     | ON         | safe   |
| true             | present    | ON         | safe   |

Config `enabled = true` always promotes to safe tier,
regardless of --unsafe flag.
