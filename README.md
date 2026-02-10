# cctidy

A CLI tool that formats Claude Code configuration files.

## Target Files

- `~/.claude.json` - Main settings with path cleaning
- `~/.claude/settings.json` - User global settings
- `~/.claude/settings.local.json` - User local settings
- `.claude/settings.json` - Project shared settings
- `.claude/settings.local.json` - Project local settings

## What It Does

- Recursive key sorting of all JSON objects
- Sorting of homogeneous arrays (string, number, bool)
- Pretty-printing with 2-space indent
- For `~/.claude.json` only:
  - Removal of non-existent project paths (`projects` key)
  - Removal of non-existent GitHub repo paths
    (`githubRepoPaths` key), including cleanup of empty
    repo keys

## Install

```bash
brew install 708u/tap/cctidy
```

Or download from
[Releases](https://github.com/708u/cctidy/releases).

## Usage

```bash
cctidy              # Format all 5 target files
cctidy -t FILE      # Format a specific file only
cctidy --dry-run    # Show changes without writing
cctidy --backup     # Create backup before writing
```

## License

[MIT](LICENSE)
