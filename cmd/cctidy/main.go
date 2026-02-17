package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/708u/cctidy"
	"github.com/708u/cctidy/internal/set"
	"github.com/alecthomas/kong"
)

var errUnformatted = errors.New("unformatted files detected")

type CLI struct {
	Target  string           `help:"Path to a specific file to format." short:"t" name:"target"`
	Backup  bool             `help:"Create backup before writing."`
	DryRun  bool             `help:"Show changes without writing." name:"dry-run"`
	Check   bool             `help:"Exit with 1 if any file needs formatting."`
	Unsafe  bool             `help:"Enable unsafe sweepers (e.g. Bash)." name:"unsafe"`
	Config  string           `help:"Path to config file." name:"config"`
	Verbose bool             `help:"Show formatting details." short:"v"`
	Version kong.VersionFlag `help:"Print version."`

	checker     cctidy.PathChecker
	cfg         *cctidy.Config
	homeDir     string
	projectRoot string
	w           io.Writer
}

type Formatter interface {
	Format(context.Context, []byte) (*cctidy.FormatResult, error)
}

type Sweeper interface {
	Sweep(context.Context, map[string]any) *cctidy.SweepResult
}

type targetFile struct {
	path      string
	formatter Formatter
}

type fileResult struct {
	path       string
	original   []byte
	result     *cctidy.FormatResult
	backupPath string
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(
		context.Background(), os.Interrupt, syscall.SIGTERM,
	)
	defer stop()

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cctidy: %v\n", err)
		return 1
	}

	cli := CLI{
		checker: &osPathChecker{},
		homeDir: home,
		w:       os.Stderr,
	}
	kong.Parse(&cli,
		kong.Vars{"version": versionString()},
	)

	cfg, err := cctidy.LoadConfig(cli.Config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cctidy: %v\n", err)
		return 1
	}

	cwd, _ := os.Getwd()
	cli.projectRoot = findProjectRoot(cwd)

	projectCfg, err := cctidy.LoadProjectConfig(cli.projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cctidy: %v\n", err)
		return 1
	}
	cli.cfg = cctidy.MergeConfig(cfg, projectCfg, cli.projectRoot)

	if cli.Check && (cli.Backup || cli.DryRun) {
		fmt.Fprintf(os.Stderr, "cctidy: --check cannot be combined with --backup or --dry-run\n")
		return 2
	}

	if err := cli.Run(ctx); err != nil {
		if errors.Is(err, errUnformatted) {
			return 1
		}
		fmt.Fprintf(os.Stderr, "cctidy: %v\n", err)
		return 2
	}
	return 0
}

func (c *CLI) Run(ctx context.Context) error {
	targets, err := c.resolveTargets()
	if err != nil {
		return err
	}
	return c.runTargets(ctx, targets)
}

func (c *CLI) checkFile(ctx context.Context, tf targetFile) (bool, error) {
	data, err := os.ReadFile(tf.path)
	if err != nil {
		return false, err
	}

	result, err := tf.formatter.Format(ctx, data)
	if err != nil {
		return false, fmt.Errorf("formatting %s: %w", tf.path, err)
	}

	return bytes.Equal(data, result.Data), nil
}

func (c *CLI) checkTargets(ctx context.Context, targets []targetFile) error {
	single := len(targets) == 1
	hasUnformatted := false

	for _, tf := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		formatted, err := c.checkFile(ctx, tf)
		if err != nil {
			if single || !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if !formatted {
			hasUnformatted = true
			if c.Verbose {
				fmt.Fprintf(c.w, "%s: needs formatting\n", tf.path)
			}
		}
	}

	if hasUnformatted {
		return errUnformatted
	}
	return nil
}

func (c *CLI) runTargets(ctx context.Context, targets []targetFile) error {
	if c.Check {
		return c.checkTargets(ctx, targets)
	}
	single := len(targets) == 1

	for _, tf := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		r, err := c.formatFile(ctx, tf)
		if err != nil {
			if single || !os.IsNotExist(err) {
				return err
			}
			if c.Verbose {
				fmt.Fprintf(c.w, "%s: skipped (not found)\n\n", tf.path)
			}
			continue
		}
		if c.Verbose {
			printResult(c.w, r, single)
		}
	}
	return nil
}

func (c *CLI) resolveTargets() ([]targetFile, error) {
	if c.Target == "" {
		return c.defaultTargets()
	}
	if filepath.Base(c.Target) == ".claude.json" {
		f := cctidy.NewClaudeJSONFormatter(c.checker)
		return []targetFile{{path: c.Target, formatter: f}}, nil
	}
	var opts []cctidy.SweepOption
	if filepath.Dir(c.Target) != filepath.Join(c.homeDir, ".claude") {
		projectDir := filepath.Dir(filepath.Dir(c.Target))
		opts = append(opts, cctidy.WithProjectLevel(projectDir))
	}
	if c.cfg != nil {
		opts = append(opts, cctidy.WithBashConfig(&c.cfg.Permission.Bash))
	}
	if c.Unsafe {
		opts = append(opts, cctidy.WithUnsafe())
	}
	serverSets := c.loadMCPServers()
	mcpServers := c.mcpServersForTarget(serverSets, c.Target)
	plugins := c.loadEnabledPlugins()
	opts = append(opts, cctidy.WithEnabledPlugins(plugins))
	sweeper, err := cctidy.NewPermissionSweeper(c.checker, c.homeDir, mcpServers, opts...)
	if err != nil {
		return nil, err
	}
	return []targetFile{{path: c.Target, formatter: cctidy.NewSettingsJSONFormatter(sweeper)}}, nil
}

func findProjectRoot(dir string) string {
	cur := dir
	for {
		candidate := filepath.Join(cur, ".claude")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return dir
		}
		cur = parent
	}
}

// loadMCPServers loads known MCP server names from .mcp.json and
// ~/.claude.json. Errors are printed as warnings.
func (c *CLI) loadMCPServers() *cctidy.MCPServerSets {
	servers, err := cctidy.LoadMCPServers(
		filepath.Join(c.projectRoot, ".mcp.json"),
		filepath.Join(c.homeDir, ".claude.json"),
	)
	if err != nil {
		fmt.Fprintf(c.w, "cctidy: warning: loading MCP servers: %v\n", err)
		return nil
	}
	return servers
}

// mcpServersForTarget returns the appropriate server set for the
// given target path. User-scope paths (~/.claude/) get User set;
// everything else gets Project set.
func (c *CLI) mcpServersForTarget(servers *cctidy.MCPServerSets, target string) set.Value[string] {
	claudeDir := filepath.Join(c.homeDir, ".claude")
	rel, err := filepath.Rel(claudeDir, target)
	if err == nil && filepath.IsLocal(rel) {
		return servers.ForUserScope()
	}
	return servers.ForProjectScope()
}

// loadEnabledPlugins loads enabledPlugins from all settings files.
// Returns nil when no file contains enabledPlugins (sweeper inactive).
func (c *CLI) loadEnabledPlugins() *cctidy.EnabledPlugins {
	return cctidy.LoadEnabledPlugins(
		filepath.Join(c.homeDir, ".claude", "settings.json"),
		filepath.Join(c.homeDir, ".claude", "settings.local.json"),
		filepath.Join(c.projectRoot, ".claude", "settings.json"),
		filepath.Join(c.projectRoot, ".claude", "settings.local.json"),
	)
}

func (c *CLI) defaultTargets() ([]targetFile, error) {
	projectRoot := c.projectRoot
	claude := cctidy.NewClaudeJSONFormatter(c.checker)
	var globalOpts []cctidy.SweepOption
	projectOpts := []cctidy.SweepOption{cctidy.WithProjectLevel(projectRoot)}
	if c.cfg != nil {
		bashOpt := cctidy.WithBashConfig(&c.cfg.Permission.Bash)
		globalOpts = append(globalOpts, bashOpt)
		projectOpts = append(projectOpts, bashOpt)
	}
	if c.Unsafe {
		unsafeOpt := cctidy.WithUnsafe()
		globalOpts = append(globalOpts, unsafeOpt)
		projectOpts = append(projectOpts, unsafeOpt)
	}
	serverSets := c.loadMCPServers()
	plugins := c.loadEnabledPlugins()
	pluginOpt := cctidy.WithEnabledPlugins(plugins)
	globalOpts = append(globalOpts, pluginOpt)
	projectOpts = append(projectOpts, pluginOpt)
	globalSweeper, err := cctidy.NewPermissionSweeper(c.checker, c.homeDir, serverSets.ForUserScope(), globalOpts...)
	if err != nil {
		return nil, err
	}
	projectSweeper, err := cctidy.NewPermissionSweeper(c.checker, c.homeDir, serverSets.ForProjectScope(), projectOpts...)
	if err != nil {
		return nil, err
	}
	globalSettings := cctidy.NewSettingsJSONFormatter(globalSweeper)
	projectSettings := cctidy.NewSettingsJSONFormatter(projectSweeper)
	return []targetFile{
		{path: filepath.Join(c.homeDir, ".claude.json"), formatter: claude},
		{path: filepath.Join(c.homeDir, ".claude", "settings.json"), formatter: globalSettings},
		{path: filepath.Join(c.homeDir, ".claude", "settings.local.json"), formatter: globalSettings},
		{path: filepath.Join(projectRoot, ".claude", "settings.json"), formatter: projectSettings},
		{path: filepath.Join(projectRoot, ".claude", "settings.local.json"), formatter: projectSettings},
	}, nil
}

func (c *CLI) formatFile(ctx context.Context, tf targetFile) (*fileResult, error) {
	info, err := os.Stat(tf.path)
	if err != nil {
		return nil, err
	}
	perm := info.Mode().Perm()

	data, err := os.ReadFile(tf.path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", tf.path, err)
	}

	result, err := tf.formatter.Format(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("formatting %s: %w", tf.path, err)
	}

	var backupPath string
	if !c.DryRun {
		if c.Backup {
			backupPath = fmt.Sprintf("%s.backup.%s",
				tf.path, time.Now().Format("20060102150405"))
			if err := os.WriteFile(backupPath, data, perm); err != nil {
				return nil, fmt.Errorf("creating backup: %w", err)
			}
		}
		if err := writeFile(tf.path, result.Data, perm); err != nil {
			return nil, fmt.Errorf("writing %s: %w", tf.path, err)
		}
	}

	return &fileResult{
		path:       tf.path,
		original:   data,
		result:     result,
		backupPath: backupPath,
	}, nil
}

func printResult(w io.Writer, r *fileResult, single bool) {
	if single {
		fmt.Fprint(w, r.result.Stats.Summary())
		printBackup(w, r.backupPath, "")
		return
	}
	if bytes.Equal(r.original, r.result.Data) {
		fmt.Fprintf(w, "%s:\n  (no changes)\n\n", r.path)
		return
	}
	fmt.Fprintf(w, "%s:\n", r.path)
	for _, line := range splitLines(r.result.Stats.Summary()) {
		fmt.Fprintf(w, "  %s\n", line)
	}
	printBackup(w, r.backupPath, "  ")
	fmt.Fprintln(w)
}

func printBackup(w io.Writer, backupPath, indent string) {
	if backupPath != "" {
		fmt.Fprintf(w, "%sBackup: %s\n", indent, backupPath)
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	start := 0
	for i := range len(s) {
		if s[i] == '\n' {
			line := s[start:i]
			if line != "" {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func writeFile(path string, data []byte, perm os.FileMode) error {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("resolving path: %w", err)
	}
	if err == nil {
		path = resolved
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	closed := false
	defer func() {
		if !closed {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := tmp.Chmod(perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	closed = true

	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

type osPathChecker struct{}

func (o *osPathChecker) Exists(_ context.Context, path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
