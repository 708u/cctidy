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
	"github.com/alecthomas/kong"
)

var errUnformatted = errors.New("unformatted files detected")

var version = "dev"

type CLI struct {
	Target          string           `help:"Path to a specific file to format." short:"t" name:"target"`
	Backup          bool             `help:"Create backup before writing."`
	DryRun          bool             `help:"Show changes without writing." name:"dry-run"`
	Check           bool             `help:"Exit with 1 if any file needs formatting."`
	IncludeBashTool bool             `help:"Include Bash tool entries in permission sweeping." name:"include-bash-tool"`
	Verbose         bool             `help:"Show formatting details." short:"v"`
	Version         kong.VersionFlag `help:"Print version."`

	checker cctidy.PathChecker
	w       io.Writer
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
		w:       os.Stderr,
	}
	kong.Parse(&cli,
		kong.Vars{"version": version},
	)

	if cli.Check && (cli.Backup || cli.DryRun) {
		fmt.Fprintf(os.Stderr, "cctidy: --check cannot be combined with --backup or --dry-run\n")
		return 2
	}

	if err := cli.Run(ctx, home); err != nil {
		if errors.Is(err, errUnformatted) {
			return 1
		}
		fmt.Fprintf(os.Stderr, "cctidy: %v\n", err)
		return 2
	}
	return 0
}

func (c *CLI) Run(ctx context.Context, home string) error {
	return c.runTargets(ctx, c.resolveTargets(home))
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

func (c *CLI) resolveTargets(home string) []targetFile {
	if c.Target == "" {
		return c.defaultTargets(home)
	}
	baseDir := filepath.Dir(filepath.Dir(c.Target))
	opts := []cctidy.SweepOption{cctidy.WithBaseDir(baseDir)}
	if c.IncludeBashTool {
		opts = append(opts, cctidy.WithBashSweep())
	}
	sweeper := cctidy.NewPermissionSweeper(c.checker, home, opts...)
	var f Formatter = cctidy.NewSettingsJSONFormatter(sweeper)
	if filepath.Base(c.Target) == ".claude.json" {
		f = cctidy.NewClaudeJSONFormatter(c.checker)
	}
	return []targetFile{{path: c.Target, formatter: f}}
}

func (c *CLI) defaultTargets(home string) []targetFile {
	cwd, _ := os.Getwd()
	claude := cctidy.NewClaudeJSONFormatter(c.checker)
	var globalOpts []cctidy.SweepOption
	projectOpts := []cctidy.SweepOption{cctidy.WithBaseDir(cwd)}
	if c.IncludeBashTool {
		globalOpts = append(globalOpts, cctidy.WithBashSweep())
		projectOpts = append(projectOpts, cctidy.WithBashSweep())
	}
	globalSettings := cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(c.checker, home, globalOpts...))
	projectSettings := cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(c.checker, home, projectOpts...))
	return []targetFile{
		{path: filepath.Join(home, ".claude.json"), formatter: claude},
		{path: filepath.Join(home, ".claude", "settings.json"), formatter: globalSettings},
		{path: filepath.Join(home, ".claude", "settings.local.json"), formatter: globalSettings},
		{path: filepath.Join(cwd, ".claude", "settings.json"), formatter: projectSettings},
		{path: filepath.Join(cwd, ".claude", "settings.local.json"), formatter: projectSettings},
	}
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
