# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code)
when working with code in this repository.

## Project Overview

ccfmt is a CLI tool that formats Claude Code configuration files:

- `~/.claude.json` - Main settings with path cleaning
- `~/.claude/settings.json` - User global settings
- `~/.claude/settings.local.json` - User local settings
- `.claude/settings.json` - Project shared settings
- `.claude/settings.local.json` - Project local settings

It performs:

- Recursive key sorting of all JSON objects
- Sorting of homogeneous arrays (string, number, bool)
- Pretty-printing with 2-space indent
- For `~/.claude.json` only:
  - Removal of non-existent project paths (`projects` key)
  - Removal of non-existent GitHub repo paths
    (`githubRepoPaths` key), including cleanup of empty
    repo keys

## Commands

```bash
make build    # Build binary to ./ccfmt
make install  # Install to $GOPATH/bin
make test     # Run all tests (unit + integration)

# Update golden files for integration tests
go test -tags integration ./cmd/ -update
```

## Architecture

Two packages:

- **`ccfmt` (root)** - Library package.
  - `Formatter.Format()` takes a `PathChecker` interface
    and raw JSON bytes. Performs path cleaning + formatting.
    Used for `~/.claude.json`.
  - `FormatJSON()` standalone function. Sorts keys and
    arrays without path cleaning. Used for settings files.
  - Both return `*FormatResult` (formatted bytes + `Stats`).
- **`cmd/`** - CLI entrypoint (`package main`). Uses kong
  for flag parsing. Processes multiple target files with
  `runAll()` / `runOne()`.

`PathChecker` interface enables testing without real
filesystem access. Tests use `alwaysTrue`, `alwaysFalse`,
and `pathSet` stubs.

Integration tests use `//go:build integration` tag and
live in `cmd/integration_test.go`. Golden test data is in
`cmd/testdata/`.

## CLI Usage

```bash
ccfmt              # Format all 5 target files
ccfmt -t FILE      # Format a specific file only
ccfmt --dry-run    # Show changes without writing
ccfmt --backup     # Create backup before writing
```
