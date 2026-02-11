package cctidy

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

var absPathRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.~])/[A-Za-z0-9_./-]+`)

var toolEntryRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\((.*)\)$`)

// extractToolEntry splits a permission entry like "Read(/path/to/file)"
// into tool name and specifier. Returns ("", "") if the entry has no specifier.
func extractToolEntry(entry string) (toolName, specifier string) {
	m := toolEntryRe.FindStringSubmatch(entry)
	if m == nil {
		return "", ""
	}
	return m[1], m[2]
}

// ToolSweepResult holds the result of a single tool sweeper evaluation.
// When Warn is non-empty the entry is kept and the warning is recorded.
type ToolSweepResult struct {
	Sweep bool
	Warn  string
}

// ToolSweeper decides whether a specifier for a specific tool should be swept.
type ToolSweeper interface {
	ShouldSweep(ctx context.Context, specifier string) ToolSweepResult
}

// ToolName identifies a Claude Code tool for permission matching.
type ToolName string

const (
	ToolRead  ToolName = "Read"
	ToolEdit  ToolName = "Edit"
	ToolWrite ToolName = "Write"
	ToolBash  ToolName = "Bash"
)

// ReadEditToolSweeper sweeps Read/Edit/Write permission entries
// that reference non-existent paths.
//
// Specifier resolution rules:
//   - glob (*, ?, [)  → skip (kept unchanged)
//   - //path          → /path  (absolute; always resolvable)
//   - ~/path          → homeDir/path (requires homeDir)
//   - /path           → project root relative (requires baseDir)
//   - ./path, ../path, bare path → cwd relative (requires baseDir)
type ReadEditToolSweeper struct {
	checker PathChecker
	homeDir string
	baseDir string
}

// containsGlob reports whether s contains glob metacharacters.
func containsGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func (r *ReadEditToolSweeper) ShouldSweep(ctx context.Context, specifier string) ToolSweepResult {
	if containsGlob(specifier) {
		return ToolSweepResult{}
	}

	var resolved string
	switch {
	case strings.HasPrefix(specifier, "//"):
		resolved = specifier[1:]
	case strings.HasPrefix(specifier, "~/"):
		if r.homeDir == "" {
			return ToolSweepResult{}
		}
		resolved = filepath.Join(r.homeDir, specifier[2:])
	default: // /path, ./path, ../path, bare path — all project-relative
		if r.baseDir == "" {
			return ToolSweepResult{}
		}
		resolved = filepath.Join(r.baseDir, specifier)
	}

	if !r.checker.Exists(ctx, resolved) {
		return ToolSweepResult{Sweep: true}
	}
	return ToolSweepResult{}
}

// extractAbsolutePaths extracts all absolute paths from a string.
// Paths stop at glob metacharacters, shell metacharacters, whitespace,
// parentheses, dollar signs, and braces.
func extractAbsolutePaths(s string) []string {
	matches := absPathRe.FindAllString(s, -1)
	var paths []string
	for _, m := range matches {
		// The regex may include a leading non-path char from the
		// lookbehind alternative; trim to the first '/'.
		idx := strings.IndexByte(m, '/')
		m = m[idx:]
		cleaned := strings.TrimRight(m, "/.")
		if cleaned == "" || cleaned == "/" {
			continue
		}
		paths = append(paths, cleaned)
	}
	return paths
}

// BashToolSweeper sweeps Bash permission entries where all
// absolute paths in the specifier are non-existent.
// Entries with no absolute paths or at least one existing path are kept.
type BashToolSweeper struct {
	checker PathChecker
}

func (b *BashToolSweeper) ShouldSweep(ctx context.Context, specifier string) ToolSweepResult {
	paths := extractAbsolutePaths(specifier)
	if len(paths) == 0 {
		return ToolSweepResult{}
	}
	for _, p := range paths {
		if b.checker.Exists(ctx, p) {
			return ToolSweepResult{}
		}
	}
	return ToolSweepResult{Sweep: true}
}

// SweepResult holds statistics from permission sweeping.
// Deny entries are intentionally excluded from sweeping because they represent
// explicit user prohibitions; removing stale deny rules costs nothing but
// could silently re-enable a previously blocked action.
type SweepResult struct {
	SweptAllow int
	SweptAsk   int
	Warns      []string
}

// sweepCategory pairs a permission category key with its swept count.
type sweepCategory struct {
	key   string
	count int
}

// PermissionSweeper sweeps stale permission entries from settings objects.
// It dispatches to tool-specific ToolSweeper implementations based on the
// tool name extracted from each entry. Entries for unregistered tools are
// kept unchanged.
//
// Ref: https://code.claude.com/docs/en/permissions#permission-rule-syntax
type PermissionSweeper struct {
	tools map[ToolName]ToolSweeper
}

// SweepOption configures a PermissionSweeper.
type SweepOption func(*sweepConfig)

type sweepConfig struct {
	baseDir   string
	bashSweep bool
}

// WithBaseDir sets the base directory for resolving relative path specifiers.
func WithBaseDir(dir string) SweepOption {
	return func(c *sweepConfig) {
		c.baseDir = dir
	}
}

// WithBashSweep enables sweeping of Bash permission entries whose
// absolute paths are all non-existent.
func WithBashSweep() SweepOption {
	return func(c *sweepConfig) {
		c.bashSweep = true
	}
}

// NewPermissionSweeper creates a PermissionSweeper.
// homeDir is required for resolving ~/path specifiers.
func NewPermissionSweeper(checker PathChecker, homeDir string, opts ...SweepOption) *PermissionSweeper {
	var cfg sweepConfig
	for _, o := range opts {
		o(&cfg)
	}

	re := &ReadEditToolSweeper{
		checker: checker,
		homeDir: homeDir,
		baseDir: cfg.baseDir,
	}

	tools := map[ToolName]ToolSweeper{
		ToolRead: re, ToolEdit: re, ToolWrite: re,
	}
	if cfg.bashSweep {
		tools[ToolBash] = &BashToolSweeper{checker: checker}
	}

	return &PermissionSweeper{tools: tools}
}

// Sweep removes stale allow/ask permission entries from obj.
func (p *PermissionSweeper) Sweep(ctx context.Context, obj map[string]any) *SweepResult {
	result := &SweepResult{}

	raw, ok := obj["permissions"]
	if !ok {
		return result
	}
	perms, ok := raw.(map[string]any)
	if !ok {
		return result
	}

	categories := []sweepCategory{
		{key: "allow"},
		{key: "ask"},
	}

	for i, cat := range categories {
		raw, ok := perms[cat.key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}

		// Filter out swept entries and replace the original array.
		kept := make([]any, 0, len(arr))
		for _, v := range arr {
			entry, ok := v.(string)
			if ok && p.shouldSweep(ctx, entry, result) {
				categories[i].count++
				continue
			}
			kept = append(kept, v)
		}
		perms[cat.key] = kept
	}

	result.SweptAllow = categories[0].count
	result.SweptAsk = categories[1].count
	return result
}

func (p *PermissionSweeper) shouldSweep(ctx context.Context, entry string, result *SweepResult) bool {
	toolName, specifier := extractToolEntry(entry)
	if toolName == "" {
		return false
	}

	sweeper, ok := p.tools[ToolName(toolName)]
	if !ok {
		return false
	}

	r := sweeper.ShouldSweep(ctx, specifier)
	if r.Warn != "" {
		result.Warns = append(result.Warns, entry)
		return false
	}
	return r.Sweep
}
