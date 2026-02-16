package cctidy

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/708u/cctidy/internal/set"
)

// absPathRe matches absolute paths starting with / in a command string.
// Used by extractAbsolutePaths to find paths like /home/user/repo.
// The leading alternative handles word boundaries without lookbehind.
var absPathRe = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.~])/[A-Za-z0-9_./-]+`)

// relPathRe matches relative paths prefixed with ./, ../, or ~/
// in a command string. Bare relative paths (e.g. src/file) are
// intentionally excluded to avoid false positives.
// Capture group 1 contains the path including its prefix.
var relPathRe = regexp.MustCompile(`(?:^|\s|=)(\.\./[A-Za-z0-9_./-]+|\./[A-Za-z0-9_./-]+|~/[A-Za-z0-9_./-]+)`)

// toolEntryRe matches a permission entry like "Read(/path/to/file)"
// and captures the tool name and specifier.
var toolEntryRe = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9_]*)\((.*)\)$`)

// ToolEntry represents a parsed permission entry routed to a specific tool.
type ToolEntry interface {
	Name() ToolName
}

// StandardEntry is a parsed Tool(specifier) permission entry.
type StandardEntry struct {
	Tool      ToolName
	Specifier string
}

func (e StandardEntry) Name() ToolName { return e.Tool }

// MCPEntry is a parsed mcp__server__tool permission entry.
type MCPEntry struct {
	ServerName string
	RawEntry   string
}

func (e MCPEntry) Name() ToolName { return ToolMCP }

// extractToolEntry parses a permission entry string into a ToolEntry.
// Returns nil for unrecognized entries.
func extractToolEntry(entry string) ToolEntry {
	if strings.HasPrefix(entry, "mcp__") {
		serverName, ok := extractMCPServerName(entry)
		if !ok {
			return nil
		}
		return MCPEntry{ServerName: serverName, RawEntry: entry}
	}
	m := toolEntryRe.FindStringSubmatch(entry)
	if m == nil {
		return nil
	}
	return StandardEntry{Tool: ToolName(m[1]), Specifier: m[2]}
}

// ToolSweepResult holds the result of a single tool sweeper evaluation.
// When Warn is non-empty the entry is kept and the warning is recorded.
type ToolSweepResult struct {
	Sweep bool
	Warn  string
}

// ToolSweeper decides whether a permission entry should be swept.
type ToolSweeper interface {
	ShouldSweep(ctx context.Context, entry ToolEntry) ToolSweepResult
}

// typedSweeper is a generic adapter that dispatches to a concrete
// ToolEntry type, returning a zero result for non-matching types.
type typedSweeper[E ToolEntry] struct {
	sweep func(context.Context, E) ToolSweepResult
}

func (s *typedSweeper[E]) ShouldSweep(ctx context.Context, entry ToolEntry) ToolSweepResult {
	e, ok := entry.(E)
	if !ok {
		return ToolSweepResult{}
	}
	return s.sweep(ctx, e)
}

// NewToolSweeper wraps a concrete-typed sweep function as a ToolSweeper.
func NewToolSweeper[E ToolEntry](fn func(context.Context, E) ToolSweepResult) ToolSweeper {
	return &typedSweeper[E]{sweep: fn}
}

// ToolName identifies a Claude Code tool for permission matching.
type ToolName string

const (
	ToolRead  ToolName = "Read"
	ToolEdit  ToolName = "Edit"
	ToolWrite ToolName = "Write"
	ToolBash  ToolName = "Bash"
	ToolTask  ToolName = "Task"
	ToolSkill ToolName = "Skill"
	ToolMCP   ToolName = "mcp"
)

// builtinAgents maps agent names that are always available
// in Claude Code and should never be swept.
var builtinAgents = set.New(
	"Bash",
	"Explore",
	"Plan",
	"claude-code-guide",
	"general-purpose",
	"statusline-setup",
)

// ReadEditToolSweeper sweeps Read/Edit permission entries
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

func (r *ReadEditToolSweeper) ShouldSweep(ctx context.Context, entry StandardEntry) ToolSweepResult {
	specifier := entry.Specifier
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
		rest, _ := strings.CutPrefix(specifier, "~/")
		resolved = filepath.Join(r.homeDir, rest)
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
		cleaned := filepath.Clean(m[idx:])
		if cleaned == "/" {
			continue
		}
		paths = append(paths, cleaned)
	}
	return paths
}

// extractRelativePaths extracts all relative paths (../, ./, ~/) from a string.
func extractRelativePaths(s string) []string {
	matches := relPathRe.FindAllStringSubmatch(s, -1)
	var paths []string
	for _, m := range matches {
		cleaned := strings.TrimRight(m[1], "/")
		if cleaned == "" {
			continue
		}
		paths = append(paths, cleaned)
	}
	return paths
}

// BashExcluder decides whether a Bash permission specifier should be
// excluded from sweeping (i.e. always kept).
type BashExcluder struct {
	entries  set.Value[string]
	commands set.Value[string]
	paths    []string // prefix match
}

// NewBashExcluder builds a BashExcluder from a BashSweepConfig.
func NewBashExcluder(cfg BashSweepConfig) *BashExcluder {
	entries := set.New(cfg.ExcludeEntries...)
	commands := set.New(cfg.ExcludeCommands...)
	paths := make([]string, len(cfg.ExcludePaths))
	for i, p := range cfg.ExcludePaths {
		paths[i] = filepath.Clean(p)
	}
	return &BashExcluder{
		entries:  entries,
		commands: commands,
		paths:    paths,
	}
}

// IsExcluded reports whether the specifier matches any exclusion rule.
// Checks are applied in order: entries (exact), commands (first token),
// paths (prefix match on pre-extracted absolute paths).
func (e *BashExcluder) IsExcluded(specifier string, absPaths []string) bool {
	if e.entries.Has(specifier) {
		return true
	}
	cmd, _, _ := strings.Cut(specifier, " ")
	if e.commands.Has(cmd) {
		return true
	}
	for _, absPath := range absPaths {
		for _, prefix := range e.paths {
			// absPath is under prefix when the relative path
			// stays local (no ".." escape).
			rel, err := filepath.Rel(prefix, absPath)
			if err == nil && filepath.IsLocal(rel) {
				return true
			}
		}
	}
	return false
}

// BashToolSweeper sweeps Bash permission entries where all
// resolvable paths in the specifier are non-existent.
// Entries with no resolvable paths or at least one existing path are kept.
type BashToolSweeper struct {
	checker  PathChecker
	homeDir  string
	baseDir  string
	excluder *BashExcluder
	active   bool
}

// NewBashToolSweeper creates a BashToolSweeper.
// active controls whether sweeping is performed at all;
// when false, ShouldSweep always returns a zero result.
func NewBashToolSweeper(checker PathChecker, homeDir, baseDir string, excluder *BashExcluder, active bool) *BashToolSweeper {
	return &BashToolSweeper{
		checker:  checker,
		homeDir:  homeDir,
		baseDir:  baseDir,
		excluder: excluder,
		active:   active,
	}
}

func (b *BashToolSweeper) ShouldSweep(ctx context.Context, entry StandardEntry) ToolSweepResult {
	if !b.active {
		return ToolSweepResult{}
	}
	specifier := entry.Specifier
	absPaths := extractAbsolutePaths(specifier)

	if b.excluder.IsExcluded(specifier, absPaths) {
		return ToolSweepResult{}
	}
	relPaths := extractRelativePaths(specifier)

	// Resolve relative paths to absolute paths.
	var resolved []string
	for _, p := range relPaths {
		switch {
		case strings.HasPrefix(p, "~/"):
			if b.homeDir == "" {
				continue
			}
			rest, _ := strings.CutPrefix(p, "~/")
			resolved = append(resolved, filepath.Join(b.homeDir, rest))
		case strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../"):
			if b.baseDir == "" {
				continue
			}
			resolved = append(resolved, filepath.Join(b.baseDir, p))
		}
	}

	allPaths := make([]string, 0, len(absPaths)+len(resolved))
	allPaths = append(allPaths, absPaths...)
	allPaths = append(allPaths, resolved...)
	if len(allPaths) == 0 {
		return ToolSweepResult{}
	}

	for _, p := range allPaths {
		if b.checker.Exists(ctx, p) {
			return ToolSweepResult{}
		}
	}
	return ToolSweepResult{Sweep: true}
}

// TaskToolSweeper sweeps Task permission entries where the
// referenced agent no longer exists. Built-in agents, plugin
// agents (containing ":"), and agents whose name appears in
// the agent name set are always kept.
type TaskToolSweeper struct {
	agents set.Value[string]
}

// NewTaskToolSweeper creates a TaskToolSweeper.
func NewTaskToolSweeper(agents set.Value[string]) *TaskToolSweeper {
	return &TaskToolSweeper{agents: agents}
}

func (t *TaskToolSweeper) ShouldSweep(_ context.Context, entry StandardEntry) ToolSweepResult {
	specifier := entry.Specifier
	if builtinAgents.Has(specifier) {
		return ToolSweepResult{}
	}
	// Plugin agents use "plugin-name:agent-name" convention
	// and are managed by the plugin system, not by .md files.
	if strings.Contains(specifier, ":") {
		return ToolSweepResult{}
	}
	if t.agents.Len() == 0 {
		return ToolSweepResult{}
	}
	if t.agents.Has(specifier) {
		return ToolSweepResult{}
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
	baseDir      string
	unsafe       bool
	bashSweepCfg *BashSweepConfig
}

// WithBaseDir sets the base directory for resolving relative path specifiers.
func WithBaseDir(dir string) SweepOption {
	return func(c *sweepConfig) {
		c.baseDir = dir
	}
}

// WithBashConfig sets the BashSweepConfig for Bash sweeping.
// Exclude patterns filter entries from sweeping.
// When cfg.Enabled is true, Bash sweep runs without --unsafe.
func WithBashConfig(cfg *BashSweepConfig) SweepOption {
	return func(c *sweepConfig) {
		c.bashSweepCfg = cfg
	}
}

// WithUnsafe enables unsafe-tier sweepers.
func WithUnsafe() SweepOption {
	return func(c *sweepConfig) {
		c.unsafe = true
	}
}

// NewPermissionSweeper creates a PermissionSweeper.
// homeDir is required for resolving ~/path specifiers.
// servers is the set of known MCP server names for MCP sweep.
func NewPermissionSweeper(checker PathChecker, homeDir string, servers set.Value[string], opts ...SweepOption) *PermissionSweeper {
	var cfg sweepConfig
	for _, o := range opts {
		o(&cfg)
	}

	re := &ReadEditToolSweeper{
		checker: checker,
		homeDir: homeDir,
		baseDir: cfg.baseDir,
	}

	mcp := NewMCPToolSweeper(servers)

	var claudeDir string
	if cfg.baseDir != "" {
		claudeDir = filepath.Join(cfg.baseDir, ".claude")
	} else if homeDir != "" {
		claudeDir = filepath.Join(homeDir, ".claude")
	}

	var agentsDir string
	if claudeDir != "" {
		agentsDir = filepath.Join(claudeDir, "agents")
	}
	task := NewTaskToolSweeper(LoadAgentNames(agentsDir))
	skill := NewSkillToolSweeper(LoadSkillNames(claudeDir))

	var bashCfg BashSweepConfig
	if cfg.bashSweepCfg != nil {
		bashCfg = *cfg.bashSweepCfg
	}
	bash := NewBashToolSweeper(
		checker, homeDir, cfg.baseDir,
		NewBashExcluder(bashCfg),
		bashCfg.Enabled || cfg.unsafe,
	)

	tools := map[ToolName]ToolSweeper{
		ToolRead:  NewToolSweeper(re.ShouldSweep),
		ToolEdit:  NewToolSweeper(re.ShouldSweep),
		ToolBash:  NewToolSweeper(bash.ShouldSweep),
		ToolMCP:   NewToolSweeper(mcp.ShouldSweep),
		ToolTask:  NewToolSweeper(task.ShouldSweep),
		ToolSkill: NewToolSweeper(skill.ShouldSweep),
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
	te := extractToolEntry(entry)
	if te == nil {
		return false
	}

	tool, ok := p.tools[te.Name()]
	if !ok {
		return false
	}

	r := tool.ShouldSweep(ctx, te)
	if r.Warn != "" {
		result.Warns = append(result.Warns, entry)
		return false
	}
	return r.Sweep
}
