package cctidy

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
)

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

// ToolPruneResult holds the result of a single tool pruner evaluation.
// When Warn is non-empty the entry is kept and the warning is recorded.
type ToolPruneResult struct {
	Prune bool
	Warn  string
}

// ToolPruner decides whether a specifier for a specific tool should be pruned.
type ToolPruner interface {
	ShouldPrune(ctx context.Context, specifier string) ToolPruneResult
}

// ReadEditToolPruner prunes Read/Edit/Write permission entries
// that reference non-existent paths.
//
// Specifier resolution rules:
//   - glob (*, ?, [)  → skip (kept unchanged)
//   - //path          → /path  (absolute; always resolvable)
//   - ~/path          → homeDir/path (requires homeDir)
//   - ./path, path, /path → baseDir/path (requires baseDir)
type ReadEditToolPruner struct {
	checker PathChecker
	homeDir string
	baseDir string
}

// containsGlob reports whether s contains glob metacharacters.
func containsGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func (r *ReadEditToolPruner) ShouldPrune(ctx context.Context, specifier string) ToolPruneResult {
	if containsGlob(specifier) {
		return ToolPruneResult{}
	}

	var resolved string
	switch {
	case strings.HasPrefix(specifier, "//"):
		resolved = specifier[1:]
	case strings.HasPrefix(specifier, "~/"):
		if r.homeDir == "" {
			return ToolPruneResult{}
		}
		resolved = filepath.Join(r.homeDir, specifier[2:])
	default:
		if r.baseDir == "" {
			return ToolPruneResult{}
		}
		resolved = filepath.Join(r.baseDir, specifier)
	}

	if !r.checker.Exists(ctx, resolved) {
		return ToolPruneResult{Prune: true}
	}
	return ToolPruneResult{}
}

// PruneResult holds statistics from permission pruning.
// Deny entries are intentionally excluded from pruning because they represent
// explicit user prohibitions; removing stale deny rules costs nothing but
// could silently re-enable a previously blocked action.
type PruneResult struct {
	PrunedAllow int
	PrunedAsk   int
	Warns       []string
}

// prunerConfig collects options before building a PermissionPruner.
type prunerConfig struct {
	homeDir string
	baseDir string
}

// PermissionPruner prunes stale permission entries from settings objects.
// It dispatches to tool-specific ToolPruner implementations based on the
// tool name extracted from each entry. Entries for unregistered tools are
// kept unchanged.
type PermissionPruner struct {
	tools  map[string]ToolPruner
	result *PruneResult
}

// PruneOption configures a PermissionPruner.
type PruneOption func(*prunerConfig)

// WithHomeDir sets the home directory for resolving ~/path specifiers.
func WithHomeDir(dir string) PruneOption {
	return func(c *prunerConfig) {
		c.homeDir = dir
	}
}

// WithBaseDir sets the base directory for resolving relative path specifiers.
func WithBaseDir(dir string) PruneOption {
	return func(c *prunerConfig) {
		c.baseDir = dir
	}
}

// NewPermissionPruner creates a PermissionPruner.
func NewPermissionPruner(checker PathChecker, opts ...PruneOption) *PermissionPruner {
	cfg := &prunerConfig{}
	for _, o := range opts {
		o(cfg)
	}

	re := &ReadEditToolPruner{
		checker: checker,
		homeDir: cfg.homeDir,
		baseDir: cfg.baseDir,
	}

	return &PermissionPruner{
		tools: map[string]ToolPruner{
			"Read": re, "Edit": re, "Write": re,
		},
	}
}

// Prune removes stale allow/ask permission entries from obj.
func (p *PermissionPruner) Prune(ctx context.Context, obj map[string]any) *PruneResult {
	p.result = &PruneResult{}

	raw, ok := obj["permissions"]
	if !ok {
		return p.result
	}
	perms, ok := raw.(map[string]any)
	if !ok {
		return p.result
	}

	type category struct {
		key   string
		count *int
	}
	categories := []category{
		{"allow", &p.result.PrunedAllow},
		{"ask", &p.result.PrunedAsk},
	}

	for _, cat := range categories {
		raw, ok := perms[cat.key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}

		kept := make([]any, 0, len(arr))
		for _, v := range arr {
			entry, ok := v.(string)
			if !ok {
				kept = append(kept, v)
				continue
			}

			if p.shouldPrune(ctx, entry) {
				*cat.count++
				continue
			}

			kept = append(kept, v)
		}
		perms[cat.key] = kept
	}

	return p.result
}

func (p *PermissionPruner) shouldPrune(ctx context.Context, entry string) bool {
	toolName, specifier := extractToolEntry(entry)
	if toolName == "" {
		return false
	}

	pruner, ok := p.tools[toolName]
	if !ok {
		return false
	}

	result := pruner.ShouldPrune(ctx, specifier)
	if result.Warn != "" {
		p.result.Warns = append(p.result.Warns, entry)
		return false
	}
	return result.Prune
}
