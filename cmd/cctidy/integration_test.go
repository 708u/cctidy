//go:build integration

package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/cctidy"
	"github.com/708u/cctidy/internal/set"
	"github.com/708u/cctidy/internal/testutil"
)

var update = flag.Bool("update", false, "update golden files")

func TestGolden(t *testing.T) {
	t.Parallel()
	input, err := os.ReadFile("testdata/input.json")
	if err != nil {
		t.Fatalf("reading input: %v", err)
	}

	f := cctidy.NewClaudeJSONFormatter(testutil.AllPathsExist{})
	result, err := f.Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	goldenPath := "testdata/golden.json"
	if *update {
		if err := os.WriteFile(goldenPath, result.Data, 0o644); err != nil {
			t.Fatalf("updating golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to generate): %v", err)
	}

	if !bytes.Equal(result.Data, golden) {
		t.Errorf("output differs from golden:\ngot:\n%s\nwant:\n%s", result.Data, golden)
	}
}

func TestSettingsGolden(t *testing.T) {
	t.Parallel()
	input, err := os.ReadFile("testdata/settings_input.json")
	if err != nil {
		t.Fatalf("reading input: %v", err)
	}

	homeDir := filepath.Join(t.TempDir(), "home")
	baseDir := filepath.Join(t.TempDir(), "project")
	// Create an agents directory with a stub agent so the agent set
	// is non-empty and unknown agents get swept.
	agentsDir := filepath.Join(baseDir, ".claude", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "stub.md"), []byte("---\nname: stub\n---\n# Stub"), 0o644)
	// Create a skills directory with a stub skill and a commands
	// directory with a stub command so the skill set is non-empty.
	skillsDir := filepath.Join(baseDir, ".claude", "skills", "stub-skill")
	os.MkdirAll(skillsDir, 0o755)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Stub Skill"), 0o644)
	commandsDir := filepath.Join(baseDir, ".claude", "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "stub-cmd.md"), []byte("# Stub Command"), 0o644)
	checker := testutil.CheckerFor(
		"/alive/repo",
		"/alive/data/file.txt",
		filepath.Join(baseDir, "bin/run"),
		filepath.Join(homeDir, "config.json"),
		filepath.Join(homeDir, "alive/notes.md"),
		filepath.Join(baseDir, "src/alive.go"),
		filepath.Join(baseDir, "../alive/output.txt"),
	)
	mcpServers := set.New("github")
	sweeper := cctidy.NewPermissionSweeper(checker, homeDir, mcpServers,
		cctidy.WithBashSweep(cctidy.BashSweepConfig{}),
		cctidy.WithBaseDir(baseDir),
	)
	result, err := cctidy.NewSettingsJSONFormatter(sweeper).Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	goldenPath := "testdata/settings_golden.json"
	if *update {
		if err := os.WriteFile(goldenPath, result.Data, 0o644); err != nil {
			t.Fatalf("updating golden: %v", err)
		}
		t.Log("settings golden file updated")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to generate): %v", err)
	}

	if !bytes.Equal(result.Data, golden) {
		t.Errorf("output differs from golden:\ngot:\n%s\nwant:\n%s", result.Data, golden)
	}
}

func TestIntegrationPathCleaning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	existingProject := filepath.Join(dir, "project-a")
	existingRepo := filepath.Join(dir, "repo-path")
	os.Mkdir(existingProject, 0o755)
	os.Mkdir(existingRepo, 0o755)

	goneProject := filepath.Join(dir, "gone-project")
	goneRepo := filepath.Join(dir, "gone-repo")

	input := `{
  "projects": {
    "` + existingProject + `": {"key": "value"},
    "` + goneProject + `": {"key": "value"}
  },
  "githubRepoPaths": {
    "org/repo-a": ["` + existingRepo + `", "` + goneRepo + `"],
    "org/repo-b": ["` + goneRepo + `"]
  }
}`

	file := filepath.Join(dir, ".claude.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, checker: &osPathChecker{}, homeDir: dir, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	if !strings.Contains(got, existingProject) {
		t.Error("existing project path was removed")
	}
	if strings.Contains(got, goneProject) {
		t.Error("non-existent project path was not removed")
	}

	if !strings.Contains(got, existingRepo) {
		t.Error("existing repo path was removed")
	}
	if strings.Contains(got, goneRepo) {
		t.Error("non-existent repo path was not removed")
	}

	if strings.Contains(got, "repo-b") {
		t.Error("empty repo key was not removed")
	}

	output := buf.String()
	if !strings.Contains(output, "Projects: 2 -> 1 (removed 1)") {
		t.Errorf("unexpected projects output: %s", output)
	}
	if !strings.Contains(output, "removed 2 paths, 1 empty repos") {
		t.Errorf("unexpected repo paths output: %s", output)
	}
}

func TestRunSingleTarget(t *testing.T) {
	t.Parallel()
	input := `{"z": 1, "a": 2}`
	wantJSON := "{\n  \"a\": 2,\n  \"githubRepoPaths\": {},\n  \"projects\": {},\n  \"z\": 1\n}\n"

	t.Run("normal flow writes file without backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Verbose: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		if string(data) != wantJSON {
			t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", data, wantJSON)
		}

		matches, _ := filepath.Glob(filepath.Join(dir, ".claude.json.backup.*"))
		if len(matches) != 0 {
			t.Errorf("backup created without --backup flag")
		}

		output := buf.String()
		if !strings.Contains(output, "Size:") {
			t.Errorf("output missing size line: %s", output)
		}
	})

	t.Run("dry-run does not modify file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, DryRun: true, Verbose: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		if string(data) != input {
			t.Errorf("file was modified in dry-run mode")
		}

		matches, _ := filepath.Glob(filepath.Join(dir, ".claude.json.backup.*"))
		if len(matches) != 0 {
			t.Errorf("backup created in dry-run mode")
		}

		output := buf.String()
		if strings.Contains(output, "Backup:") {
			t.Errorf("dry-run output should not contain backup line")
		}
	})

	t.Run("backup flag creates backup", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Backup: true, Verbose: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		if string(data) != wantJSON {
			t.Errorf("file content mismatch")
		}

		matches, _ := filepath.Glob(filepath.Join(dir, ".claude.json.backup.*"))
		if len(matches) != 1 {
			t.Fatalf("expected 1 backup file, got %d", len(matches))
		}
		backup, _ := os.ReadFile(matches[0])
		if string(backup) != input {
			t.Errorf("backup content mismatch: got %q, want %q", backup, input)
		}

		output := buf.String()
		if !strings.Contains(output, "Backup:") {
			t.Errorf("output missing backup line: %s", output)
		}
	})

	t.Run("no temp files remain after write", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(`{"z": 1}`), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		matches, _ := filepath.Glob(filepath.Join(dir, ".claude.json.tmp.*"))
		if len(matches) != 0 {
			t.Errorf("temp files remain: %v", matches)
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(input), 0o600)

		var buf bytes.Buffer
		cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, _ := os.Stat(file)
		if info.Mode().Perm() != 0o600 {
			t.Errorf("permission changed: got %o, want 600", info.Mode().Perm())
		}
	})

	t.Run("single target file not found error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cli := &CLI{Target: "/nonexistent/path/test.json", checker: testutil.AllPathsExist{}, homeDir: "/tmp", w: &buf}
		err := cli.Run(t.Context())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !os.IsNotExist(err) {
			t.Errorf("expected os.IsNotExist error: %v", err)
		}
	})
}

func TestRunMultipleTargets(t *testing.T) {
	t.Parallel()

	t.Run("formats multiple files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{"z": 1, "a": 2}`), 0o644)

		settingsDir := filepath.Join(dir, ".claude")
		os.Mkdir(settingsDir, 0o755)
		settingsJSON := filepath.Join(settingsDir, "settings.json")
		os.WriteFile(settingsJSON, []byte(`{"permissions":{"allow":["Write","Read"]}}`), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Verbose: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(testutil.AllPathsExist{})},
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		if err := cli.runTargets(t.Context(), targets); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		claudeData, _ := os.ReadFile(claudeJSON)
		if !strings.Contains(string(claudeData), `"a": 2`) {
			t.Error("claude.json was not formatted")
		}

		settingsData, _ := os.ReadFile(settingsJSON)
		if !strings.Contains(string(settingsData), `"allow"`) {
			t.Error("settings.json was not formatted")
		}

		output := buf.String()
		if !strings.Contains(output, claudeJSON+":") {
			t.Errorf("output should contain claude.json path: %s", output)
		}
		if !strings.Contains(output, settingsJSON+":") {
			t.Errorf("output should contain settings.json path: %s", output)
		}
	})

	t.Run("skips non-existent files in multi mode", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{"z": 1, "a": 2}`), 0o644)

		missingFile := filepath.Join(dir, ".claude", "settings.json")

		var buf bytes.Buffer
		cli := &CLI{Verbose: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(testutil.AllPathsExist{})},
			{path: missingFile, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		if err := cli.runTargets(t.Context(), targets); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "skipped (not found)") {
			t.Errorf("output should contain skip message: %s", output)
		}
	})

	t.Run("no changes shown for already formatted file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		settingsJSON := filepath.Join(dir, "settings.json")
		os.WriteFile(settingsJSON, []byte(formatted), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Verbose: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
			{path: filepath.Join(dir, "missing.json"), formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		if err := cli.runTargets(t.Context(), targets); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "(no changes)") {
			t.Errorf("output should contain 'no changes': %s", output)
		}
	})

	t.Run("settings file uses SettingsJSONFormatter without path cleaning", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		input := `{"permissions":{"allow":["Write","Read"]},"env":{"Z":"z","A":"a"}}`
		settingsJSON := filepath.Join(dir, "settings.json")
		os.WriteFile(settingsJSON, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: settingsJSON, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(settingsJSON)
		got := string(data)

		if strings.Contains(got, `"projects"`) {
			t.Error("SettingsJSONFormatter should not add projects key")
		}
		if strings.Contains(got, `"githubRepoPaths"`) {
			t.Error("SettingsJSONFormatter should not add githubRepoPaths key")
		}

		if !strings.Contains(got, `"A": "a"`) {
			t.Error("env keys should be sorted")
		}
		if !strings.Contains(got, `"allow"`) {
			t.Error("permissions should be preserved")
		}
	})
}

func TestCheck(t *testing.T) {
	t.Parallel()

	t.Run("formatted file returns nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(formatted), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected no output, got: %q", buf.String())
		}
	})

	t.Run("unformatted file returns errUnformatted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(`{"b":1,"a":2}`), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		err := cli.Run(t.Context())
		if !errors.Is(err, errUnformatted) {
			t.Fatalf("expected errUnformatted, got: %v", err)
		}
	})

	t.Run("check does not modify file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		input := `{"b":1,"a":2}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		cli.Run(t.Context())

		data, _ := os.ReadFile(file)
		if string(data) != input {
			t.Errorf("file was modified in check mode")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cli := &CLI{Target: "/nonexistent/path/test.json", Check: true, checker: testutil.AllPathsExist{}, homeDir: "/tmp", w: &buf}
		err := cli.Run(t.Context())
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if errors.Is(err, errUnformatted) {
			t.Error("should not be errUnformatted")
		}
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(`{invalid json`), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
		err := cli.Run(t.Context())
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if errors.Is(err, errUnformatted) {
			t.Error("should not be errUnformatted")
		}
	})
}

func TestCheckMultipleTargets(t *testing.T) {
	t.Parallel()

	t.Run("all formatted returns nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		f1 := filepath.Join(dir, "a.json")
		f2 := filepath.Join(dir, "b.json")
		os.WriteFile(f1, []byte(formatted), 0o644)
		os.WriteFile(f2, []byte(formatted), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Check: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		if err := cli.runTargets(t.Context(), targets); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})

	t.Run("one unformatted returns errUnformatted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		unformatted := `{"b":1,"a":2}`
		f1 := filepath.Join(dir, "a.json")
		f2 := filepath.Join(dir, "b.json")
		os.WriteFile(f1, []byte(formatted), 0o644)
		os.WriteFile(f2, []byte(unformatted), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Check: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		err := cli.runTargets(t.Context(), targets)
		if !errors.Is(err, errUnformatted) {
			t.Fatalf("expected errUnformatted, got: %v", err)
		}
	})

	t.Run("skips missing files", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		f1 := filepath.Join(dir, "a.json")
		os.WriteFile(f1, []byte(formatted), 0o644)
		missing := filepath.Join(dir, "missing.json")

		var buf bytes.Buffer
		cli := &CLI{Check: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
			{path: missing, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		if err := cli.runTargets(t.Context(), targets); err != nil {
			t.Fatalf("expected nil, got: %v", err)
		}
	})

	t.Run("verbose prints unformatted paths", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		formatted := "{\n  \"a\": 1,\n  \"b\": 2\n}\n"
		unformatted := `{"b":1,"a":2}`
		f1 := filepath.Join(dir, "a.json")
		f2 := filepath.Join(dir, "b.json")
		os.WriteFile(f1, []byte(formatted), 0o644)
		os.WriteFile(f2, []byte(unformatted), 0o644)

		var buf bytes.Buffer
		cli := &CLI{Check: true, Verbose: true, homeDir: dir, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, "", nil))},
		}
		cli.runTargets(t.Context(), targets)

		output := buf.String()
		if !strings.Contains(output, f2+": needs formatting") {
			t.Errorf("expected unformatted path in output: %s", output)
		}
		if strings.Contains(output, f1+": needs formatting") {
			t.Errorf("formatted file should not appear in output: %s", output)
		}
	})
}

func TestDefaultSilentOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, ".claude.json")
	os.WriteFile(file, []byte(`{"z": 1, "a": 2}`), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, homeDir: dir, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("default output should be empty, got: %q", buf.String())
	}
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()

	t.Run("with target flag returns single target", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/some/path.json", checker: testutil.AllPathsExist{}, homeDir: "/home/user"}
		targets := cli.resolveTargets()
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].path != "/some/path.json" {
			t.Errorf("unexpected path: %s", targets[0].path)
		}
	})

	t.Run("claude.json target uses ClaudeJSONFormatter", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/home/user/.claude.json", homeDir: "/home/user", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets()
		if _, ok := targets[0].formatter.(*cctidy.ClaudeJSONFormatter); !ok {
			t.Errorf("claude.json should use *cctidy.ClaudeJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("settings.json target uses SettingsJSONFormatter", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/home/user/.claude/settings.json", homeDir: "/home/user", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets()
		if _, ok := targets[0].formatter.(*cctidy.SettingsJSONFormatter); !ok {
			t.Errorf("settings.json should use *cctidy.SettingsJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("without target returns default targets", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{homeDir: "/home/user", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets()
		if len(targets) != 5 {
			t.Fatalf("expected 5 targets, got %d", len(targets))
		}
		if targets[0].path != "/home/user/.claude.json" {
			t.Errorf("first target should be claude.json: %s", targets[0].path)
		}
		if _, ok := targets[0].formatter.(*cctidy.ClaudeJSONFormatter); !ok {
			t.Errorf("claude.json should use *cctidy.ClaudeJSONFormatter, got %T", targets[0].formatter)
		}
		for _, tf := range targets[1:] {
			if _, ok := tf.formatter.(*cctidy.SettingsJSONFormatter); !ok {
				t.Errorf("settings file %s should use *cctidy.SettingsJSONFormatter, got %T", tf.path, tf.formatter)
			}
		}
	})
}

func TestIntegrationSweep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	existingPath := filepath.Join(dir, "project-a")
	os.Mkdir(existingPath, 0o755)
	deadPath := filepath.Join(dir, "gone-project")

	input := `{
  "permissions": {
    "allow": [
      "Read(/` + existingPath + `)",
      "Read(/` + deadPath + `)",
      "Read",
      "Write"
    ],
    "deny": [
      "Bash(rm -rf ` + deadPath + `)"
    ]
  }
}`
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	if !strings.Contains(got, existingPath) {
		t.Error("existing path entry was removed")
	}
	if strings.Contains(got, `"Read(/`+deadPath) {
		t.Error("dead path entry in allow was not removed")
	}
	if !strings.Contains(got, `"Read"`) {
		t.Error("non-path entry was removed")
	}
	if !strings.Contains(got, `"Bash(rm -rf `+deadPath) {
		t.Error("deny entry with dead path was incorrectly swept")
	}

	output := buf.String()
	if !strings.Contains(output, "Swept:") {
		t.Errorf("expected swept stats in output: %s", output)
	}
}

func TestIntegrationBashSweep(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create project structure: dir/project/.claude/settings.json
	// This gives baseDir = dir/project, homeDir = dir
	projectDir := filepath.Join(dir, "project")
	claudeDir := filepath.Join(projectDir, ".claude")
	os.MkdirAll(claudeDir, 0o755)

	existingPath := filepath.Join(dir, "alive-repo")
	os.Mkdir(existingPath, 0o755)
	deadPath := filepath.Join(dir, "dead-repo")

	// Create files for alive relative paths
	os.MkdirAll(filepath.Join(projectDir, "bin"), 0o755)
	os.WriteFile(filepath.Join(projectDir, "bin", "run"), []byte(""), 0o755)
	os.MkdirAll(filepath.Join(dir, "scripts"), 0o755)
	os.WriteFile(filepath.Join(dir, "scripts", "deploy.sh"), []byte(""), 0o755)
	os.WriteFile(filepath.Join(dir, "config.json"), []byte(""), 0o644)

	input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)",
      "Bash(git -C ` + existingPath + ` status)",
      "Bash(cp ` + existingPath + ` ` + deadPath + `)",
      "Bash(npm run *)",
      "Bash(./bin/run --project)",
      "Bash(../scripts/deploy.sh)",
      "Bash(cat ~/config.json)",
      "Bash(cat ~/dead/missing.conf)",
      "Read"
    ],
    "deny": [
      "Bash(rm -rf ` + deadPath + `)"
    ]
  }
}`
	file := filepath.Join(claudeDir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, SweepBash: true, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	// dead-only absolute path bash entry should be swept
	if strings.Contains(got, `"Bash(git -C `+deadPath) {
		t.Error("bash entry with only dead absolute path was not swept")
	}
	// bash entry with alive absolute path should be kept
	if !strings.Contains(got, `"Bash(git -C `+existingPath) {
		t.Error("bash entry with alive absolute path was removed")
	}
	// bash entry with one alive absolute path (cp) should be kept
	if !strings.Contains(got, `"Bash(cp `+existingPath) {
		t.Error("bash entry with one alive absolute path was removed")
	}
	// bash entry without paths should be kept
	if !strings.Contains(got, `"Bash(npm run *)`) {
		t.Error("bash entry without paths was removed")
	}
	// dot-slash relative path with existing file should be kept
	if !strings.Contains(got, `"Bash(./bin/run --project)`) {
		t.Error("bash entry with alive dot-slash relative path was removed")
	}
	// dot-dot-slash relative path with existing file should be kept
	if !strings.Contains(got, `"Bash(../scripts/deploy.sh)`) {
		t.Error("bash entry with alive dot-dot-slash relative path was removed")
	}
	// tilde home path with existing file should be kept
	if !strings.Contains(got, `"Bash(cat ~/config.json)`) {
		t.Error("bash entry with alive tilde home path was removed")
	}
	// tilde home path with dead file should be swept
	if strings.Contains(got, `"Bash(cat ~/dead/missing.conf)`) {
		t.Error("bash entry with dead tilde home path was not swept")
	}
	// non-bash entry should be kept
	if !strings.Contains(got, `"Read"`) {
		t.Error("non-bash entry was removed")
	}
	// deny bash entry should never be swept
	if !strings.Contains(got, `"Bash(rm -rf `+deadPath) {
		t.Error("deny bash entry was incorrectly swept")
	}

	output := buf.String()
	if !strings.Contains(output, "Swept:") {
		t.Errorf("expected swept stats in output: %s", output)
	}
}

func TestIntegrationTaskSweepProjectLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create project structure: dir/project/.claude/settings.json
	projectDir := filepath.Join(dir, "project")
	claudeDir := filepath.Join(projectDir, ".claude")
	agentsDir := filepath.Join(claudeDir, "agents")
	os.MkdirAll(agentsDir, 0o755)

	// Create an agent file for alive-agent in project
	os.WriteFile(filepath.Join(agentsDir, "alive-agent.md"), []byte("---\nname: alive-agent\n---\n# Alive Agent"), 0o644)

	// Create an agent file with a frontmatter name different from filename
	os.WriteFile(filepath.Join(agentsDir, "file-name-agent.md"),
		[]byte("---\nname: frontmatter-agent\n---\n# Agent\n"), 0o644)

	// Create a home agents directory with a home-agent (not in project)
	homeAgentsDir := filepath.Join(dir, ".claude", "agents")
	os.MkdirAll(homeAgentsDir, 0o755)
	os.WriteFile(filepath.Join(homeAgentsDir, "home-agent.md"), []byte("# Home Agent"), 0o644)

	input := `{
  "permissions": {
    "allow": [
      "Task(Explore)",
      "Task(dead-agent)",
      "Task(alive-agent)",
      "Task(home-agent)",
      "Task(plugin:some-agent)",
      "Task(another-dead)",
      "Task(frontmatter-agent)",
      "Task(file-name-agent)",
      "Read"
    ],
    "deny": [
      "Task(denied-dead-agent)"
    ]
  }
}`
	file := filepath.Join(claudeDir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	// built-in agent should be kept
	if !strings.Contains(got, `"Task(Explore)"`) {
		t.Error("built-in Task(Explore) was removed")
	}
	// dead agent should be swept
	if strings.Contains(got, `"Task(dead-agent)"`) {
		t.Error("dead Task(dead-agent) was not swept")
	}
	// alive agent (project .md) should be kept
	if !strings.Contains(got, `"Task(alive-agent)"`) {
		t.Error("alive Task(alive-agent) was removed")
	}
	// home-only agent should be swept in project-level settings
	if strings.Contains(got, `"Task(home-agent)"`) {
		t.Error("home-only Task(home-agent) was not swept from project settings")
	}
	// plugin agent should be kept
	if !strings.Contains(got, `"Task(plugin:some-agent)"`) {
		t.Error("plugin Task(plugin:some-agent) was removed")
	}
	// another dead agent should be swept
	if strings.Contains(got, `"Task(another-dead)"`) {
		t.Error("dead Task(another-dead) was not swept")
	}
	// frontmatter name agent should be kept
	if !strings.Contains(got, `"Task(frontmatter-agent)"`) {
		t.Error("frontmatter-named Task(frontmatter-agent) was removed")
	}
	// file-name-agent should be swept (only frontmatter name is used)
	if strings.Contains(got, `"Task(file-name-agent)"`) {
		t.Error("filename-only Task(file-name-agent) should be swept")
	}
	// non-Task entry should be kept
	if !strings.Contains(got, `"Read"`) {
		t.Error("non-Task entry was removed")
	}
	// deny entry should never be swept
	if !strings.Contains(got, `"Task(denied-dead-agent)"`) {
		t.Error("deny Task entry was incorrectly swept")
	}

	output := buf.String()
	if !strings.Contains(output, "Swept:") {
		t.Errorf("expected swept stats in output: %s", output)
	}
}

func TestIntegrationTaskSweepUserLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create home agents directory with a home-agent
	homeAgentsDir := filepath.Join(dir, ".claude", "agents")
	os.MkdirAll(homeAgentsDir, 0o755)
	os.WriteFile(filepath.Join(homeAgentsDir, "home-agent.md"),
		[]byte("---\nname: home-agent\n---\n# Home Agent"), 0o644)

	// Create an agent file with a frontmatter name different from filename
	os.WriteFile(filepath.Join(homeAgentsDir, "fm-file.md"),
		[]byte("---\nname: home-fm-agent\n---\n# Agent\n"), 0o644)

	input := `{
  "permissions": {
    "allow": [
      "Task(Explore)",
      "Task(dead-agent)",
      "Task(home-agent)",
      "Task(plugin:some-agent)",
      "Task(home-fm-agent)",
      "Task(fm-file)"
    ]
  }
}`
	file := filepath.Join(dir, ".claude", "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	// built-in agent should be kept
	if !strings.Contains(got, `"Task(Explore)"`) {
		t.Error("built-in Task(Explore) was removed")
	}
	// dead agent should be swept
	if strings.Contains(got, `"Task(dead-agent)"`) {
		t.Error("dead Task(dead-agent) was not swept")
	}
	// home agent should be kept in user-level settings
	if !strings.Contains(got, `"Task(home-agent)"`) {
		t.Error("alive Task(home-agent) was removed from user settings")
	}
	// plugin agent should be kept
	if !strings.Contains(got, `"Task(plugin:some-agent)"`) {
		t.Error("plugin Task(plugin:some-agent) was removed")
	}
	// frontmatter name agent should be kept
	if !strings.Contains(got, `"Task(home-fm-agent)"`) {
		t.Error("frontmatter-named Task(home-fm-agent) was removed from user settings")
	}
	// filename-only agent should be swept (only frontmatter name is used)
	if strings.Contains(got, `"Task(fm-file)"`) {
		t.Error("filename-only Task(fm-file) should be swept from user settings")
	}
}

func TestIntegrationBashSweepDisabledByDefault(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	deadPath := filepath.Join(dir, "dead-repo")

	input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	// SweepBash is NOT set
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	if !strings.Contains(got, `"Bash(git -C `+deadPath) {
		t.Error("bash entry was swept without --sweep-bash flag")
	}
}

func TestSweepCheck(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	deadPath := filepath.Join(dir, "gone-project")

	input := `{
  "permissions": {
    "allow": [
      "Read(/` + deadPath + `)"
    ]
  }
}`
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Check: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	err := cli.Run(t.Context())
	if !errors.Is(err, errUnformatted) {
		t.Fatalf("expected errUnformatted, got: %v", err)
	}
}

func TestIntegrationConfigExclude(t *testing.T) {
	t.Parallel()

	t.Run("exclude_commands keeps excluded entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-dir")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
exclude_commands = ["mkdir", "touch"]
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(mkdir -p ` + deadPath + `/logs)",
      "Bash(touch ` + deadPath + `/.init)",
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:    file,
			SweepBash: true,
			Verbose:   true,
			homeDir:   dir,
			checker:   &osPathChecker{},
			cfg:       cfg,
			w:         &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(mkdir -p `+deadPath) {
			t.Error("mkdir entry should be kept by exclude_commands")
		}
		if !strings.Contains(got, `"Bash(touch `+deadPath) {
			t.Error("touch entry should be kept by exclude_commands")
		}
		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("git entry with dead path should be swept")
		}
	})

	t.Run("exclude_entries keeps exact match", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
exclude_entries = ["install -m 755 `+deadPath+`/bin/app"]
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(install -m 755 ` + deadPath + `/bin/app)",
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:    file,
			SweepBash: true,
			Verbose:   true,
			homeDir:   dir,
			checker:   &osPathChecker{},
			cfg:       cfg,
			w:         &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(install -m 755 `+deadPath) {
			t.Error("exact entry should be kept by exclude_entries")
		}
		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("git entry with dead path should be swept")
		}
	})

	t.Run("exclude_paths keeps entries with matching path prefix", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
exclude_paths = ["`+deadPath+`/opt/"]
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(cat ` + deadPath + `/opt/config.yaml)",
      "Bash(git -C ` + deadPath + `/repo status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:    file,
			SweepBash: true,
			Verbose:   true,
			homeDir:   dir,
			checker:   &osPathChecker{},
			cfg:       cfg,
			w:         &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(cat `+deadPath+`/opt/config.yaml)`) {
			t.Error("entry with excluded path prefix should be kept")
		}
		if strings.Contains(got, `"Bash(git -C `+deadPath+`/repo`) {
			t.Error("entry without excluded path prefix should be swept")
		}
	})

	t.Run("config enabled activates bash sweep without CLI flag", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
enabled = true
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:  file,
			Verbose: true,
			homeDir: dir,
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be swept when config enabled=true")
		}
	})

	t.Run("config enabled=false does not activate bash sweep", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
enabled = false
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:  file,
			Verbose: true,
			homeDir: dir,
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be kept when config enabled=false")
		}
	})

	t.Run("CLI flag overrides config enabled=false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		configDir := filepath.Join(dir, "config")
		os.MkdirAll(configDir, 0o755)
		configFile := filepath.Join(configDir, "config.toml")
		os.WriteFile(configFile, []byte(`
[sweep.bash]
enabled = false
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(configFile)
		if err != nil {
			t.Fatalf("loading config: %v", err)
		}

		var buf bytes.Buffer
		cli := &CLI{
			Target:    file,
			SweepBash: true,
			Verbose:   true,
			homeDir:   dir,
			checker:   &osPathChecker{},
			cfg:       cfg,
			w:         &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("CLI --sweep-bash should override config enabled=false")
		}
	})

	t.Run("no config file preserves existing behavior", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, _ := cctidy.LoadConfig("/nonexistent/config.toml")

		var buf bytes.Buffer
		cli := &CLI{
			Target:  file,
			Verbose: true,
			homeDir: dir,
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be kept without config or CLI flag")
		}
	})
}

func TestSweepDryRun(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	deadPath := filepath.Join(dir, "gone-project")

	input := `{
  "permissions": {
    "allow": [
      "Read(/` + deadPath + `)"
    ]
  }
}`
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, DryRun: true, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != input {
		t.Error("file was modified in dry-run mode")
	}
}

func TestIntegrationProjectConfig(t *testing.T) {
	t.Parallel()

	t.Run("project shared config enables bash sweep", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")
		projectDir := filepath.Join(dir, "project")

		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[sweep.bash]\nenabled = true\n"), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, _ := cctidy.LoadConfig("/nonexistent/config.toml")
		projectCfg, err := cctidy.LoadProjectConfig(projectDir)
		if err != nil {
			t.Fatalf("loading project config: %v", err)
		}
		merged := cctidy.MergeConfig(cfg, projectCfg, projectDir)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			cfg:         merged,
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)
		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be swept by project config enabled=true")
		}
	})

	t.Run("project local overrides shared", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")
		projectDir := filepath.Join(dir, "project")

		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[sweep.bash]\nenabled = true\n"), 0o644)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("[sweep.bash]\nenabled = false\n"), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, _ := cctidy.LoadConfig("/nonexistent/config.toml")
		projectCfg, err := cctidy.LoadProjectConfig(projectDir)
		if err != nil {
			t.Fatalf("loading project config: %v", err)
		}
		merged := cctidy.MergeConfig(cfg, projectCfg, projectDir)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			cfg:         merged,
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)
		if !strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be kept when local config disables sweep")
		}
	})

	t.Run("three layer merge: global + shared + local", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		// Global config: exclude "mkdir"
		globalCfgDir := filepath.Join(dir, "global-config")
		os.MkdirAll(globalCfgDir, 0o755)
		globalCfg := filepath.Join(globalCfgDir, "config.toml")
		os.WriteFile(globalCfg, []byte(`
[sweep.bash]
exclude_commands = ["mkdir"]
`), 0o644)

		// Project shared config: enable sweep, exclude "touch"
		claudeDir := filepath.Join(dir, "project", ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"), []byte(`
[sweep.bash]
enabled = true
exclude_commands = ["touch"]
`), 0o644)

		// Project local config: exclude "cp"
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"), []byte(`
[sweep.bash]
exclude_commands = ["cp"]
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(mkdir -p ` + deadPath + `/logs)",
      "Bash(touch ` + deadPath + `/.init)",
      "Bash(cp ` + deadPath + ` /tmp/backup)",
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, err := cctidy.LoadConfig(globalCfg)
		if err != nil {
			t.Fatalf("loading global config: %v", err)
		}
		projectCfg, err := cctidy.LoadProjectConfig(filepath.Join(dir, "project"))
		if err != nil {
			t.Fatalf("loading project config: %v", err)
		}
		merged := cctidy.MergeConfig(cfg, projectCfg, filepath.Join(dir, "project"))

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			cfg:         merged,
			projectRoot: filepath.Join(dir, "project"),
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		// mkdir excluded by global config
		if !strings.Contains(got, `"Bash(mkdir -p `+deadPath) {
			t.Error("mkdir entry should be kept by global exclude_commands")
		}
		// touch excluded by project shared config
		if !strings.Contains(got, `"Bash(touch `+deadPath) {
			t.Error("touch entry should be kept by project shared exclude_commands")
		}
		// cp excluded by project local config
		if !strings.Contains(got, `"Bash(cp `+deadPath) {
			t.Error("cp entry should be kept by project local exclude_commands")
		}
		// git not excluded, should be swept
		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("git entry with dead path should be swept")
		}
	})

	t.Run("project config relative paths resolved correctly", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		projectDir := filepath.Join(dir, "project")
		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)

		// Create a file inside the excluded relative path
		excludedDir := filepath.Join(projectDir, "vendor", "lib")
		os.MkdirAll(excludedDir, 0o755)

		// Dead path that would normally be swept
		deadPath := filepath.Join(excludedDir, "dead-file")

		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"), []byte(`
[sweep.bash]
enabled = true
exclude_paths = ["vendor/"]
`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "Bash(cat ` + deadPath + `)"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, _ := cctidy.LoadConfig("/nonexistent/config.toml")
		projectCfg, err := cctidy.LoadProjectConfig(projectDir)
		if err != nil {
			t.Fatalf("loading project config: %v", err)
		}
		merged := cctidy.MergeConfig(cfg, projectCfg, projectDir)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			cfg:         merged,
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		// Dead path under vendor/ should be kept due to exclude_paths
		if !strings.Contains(got, `"Bash(cat `+deadPath) {
			t.Error("entry under excluded relative path should be kept")
		}
	})

	t.Run("no project config preserves existing behavior", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		deadPath := filepath.Join(dir, "dead-repo")

		// No .claude/cctidy.toml files
		claudeDir := filepath.Join(dir, "project", ".claude")
		os.MkdirAll(claudeDir, 0o755)

		input := `{
  "permissions": {
    "allow": [
      "Bash(git -C ` + deadPath + ` status)"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		cfg, _ := cctidy.LoadConfig("/nonexistent/config.toml")
		projectCfg, err := cctidy.LoadProjectConfig(filepath.Join(dir, "project"))
		if err != nil {
			t.Fatalf("loading project config: %v", err)
		}
		merged := cctidy.MergeConfig(cfg, projectCfg, filepath.Join(dir, "project"))

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			cfg:         merged,
			projectRoot: filepath.Join(dir, "project"),
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("bash entry should be kept without project config")
		}
	})
}

func TestIntegrationMCPSweep(t *testing.T) {
	t.Parallel()

	t.Run("sweep stale MCP entries", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		projectDir := filepath.Join(dir, "project")
		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)

		// Create .mcp.json with only "slack" server
		os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte(`{
			"mcpServers": {
				"slack": {"type": "stdio", "command": "slack-mcp"}
			}
		}`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "mcp__slack__post_message",
      "mcp__jira__create_issue",
      "mcp__sentry__get_alert",
      "mcp__plugin_github_github__search_code",
      "Read",
      "Bash(npm run *)"
    ],
    "deny": [
      "mcp__jira__delete_issue"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		// slack server exists → keep
		if !strings.Contains(got, `"mcp__slack__post_message"`) {
			t.Error("mcp__slack entry should be kept (server exists)")
		}
		// jira server missing → sweep
		if strings.Contains(got, `"mcp__jira__create_issue"`) {
			t.Error("mcp__jira entry should be swept (server missing)")
		}
		// sentry server missing → sweep
		if strings.Contains(got, `"mcp__sentry__get_alert"`) {
			t.Error("mcp__sentry entry should be swept (server missing)")
		}
		// plugin entry → keep (not standard MCP)
		if !strings.Contains(got, `"mcp__plugin_github_github__search_code"`) {
			t.Error("plugin entry should be kept")
		}
		// non-MCP entries → keep
		if !strings.Contains(got, `"Read"`) {
			t.Error("non-MCP entry was removed")
		}
		if !strings.Contains(got, `"Bash(npm run *)"`) {
			t.Error("Bash entry was removed")
		}
		// deny MCP entry → keep
		if !strings.Contains(got, `"mcp__jira__delete_issue"`) {
			t.Error("deny MCP entry was incorrectly swept")
		}

		output := buf.String()
		if !strings.Contains(output, "Swept:") {
			t.Errorf("expected swept stats in output: %s", output)
		}
	})

	t.Run("MCP sweep with claude.json servers", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		projectDir := filepath.Join(dir, "project")
		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)

		// No .mcp.json, but ~/.claude.json has servers
		os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{
			"mcpServers": {
				"github": {"type": "stdio"}
			}
		}`), 0o644)

		input := `{
  "permissions": {
    "allow": [
      "mcp__github__search_code",
      "mcp__slack__post_message"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if !strings.Contains(got, `"mcp__github__search_code"`) {
			t.Error("github entry should be kept (in claude.json)")
		}
		if strings.Contains(got, `"mcp__slack__post_message"`) {
			t.Error("slack entry should be swept (not in any config)")
		}
	})

	t.Run("user settings not affected by mcp.json servers", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		projectDir := filepath.Join(dir, "project")
		os.MkdirAll(filepath.Join(projectDir, ".claude"), 0o755)

		// .mcp.json has "slack" only
		os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte(`{
			"mcpServers": {
				"slack": {"type": "stdio"}
			}
		}`), 0o644)

		// ~/.claude.json has "github" only
		os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{
			"mcpServers": {
				"github": {"type": "stdio"}
			}
		}`), 0o644)

		// User settings file with entries for both servers
		userClaudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(userClaudeDir, 0o755)
		input := `{
  "permissions": {
    "allow": [
      "mcp__github__search_code",
      "mcp__slack__post_message"
    ]
  }
}`
		file := filepath.Join(userClaudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		// github is in claude.json → keep in user settings
		if !strings.Contains(got, `"mcp__github__search_code"`) {
			t.Error("github entry should be kept in user settings (in claude.json)")
		}
		// slack is only in .mcp.json → sweep from user settings
		if strings.Contains(got, `"mcp__slack__post_message"`) {
			t.Error("slack entry should be swept from user settings (only in .mcp.json)")
		}
	})

	t.Run("project settings keep servers from both sources", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		projectDir := filepath.Join(dir, "project")
		claudeDir := filepath.Join(projectDir, ".claude")
		os.MkdirAll(claudeDir, 0o755)

		// .mcp.json has "slack"
		os.WriteFile(filepath.Join(projectDir, ".mcp.json"), []byte(`{
			"mcpServers": {
				"slack": {"type": "stdio"}
			}
		}`), 0o644)

		// ~/.claude.json has "github"
		os.WriteFile(filepath.Join(dir, ".claude.json"), []byte(`{
			"mcpServers": {
				"github": {"type": "stdio"}
			}
		}`), 0o644)

		// Project settings file with entries for both + unknown
		input := `{
  "permissions": {
    "allow": [
      "mcp__github__search_code",
      "mcp__slack__post_message",
      "mcp__jira__create_issue"
    ]
  }
}`
		file := filepath.Join(claudeDir, "settings.json")
		os.WriteFile(file, []byte(input), 0o644)

		var buf bytes.Buffer
		cli := &CLI{
			Target:      file,
			Verbose:     true,
			homeDir:     dir,
			checker:     &osPathChecker{},
			projectRoot: projectDir,
			w:           &buf,
		}
		if err := cli.Run(t.Context()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		// github from claude.json → keep in project settings
		if !strings.Contains(got, `"mcp__github__search_code"`) {
			t.Error("github entry should be kept in project settings")
		}
		// slack from .mcp.json → keep in project settings
		if !strings.Contains(got, `"mcp__slack__post_message"`) {
			t.Error("slack entry should be kept in project settings")
		}
		// jira not in any source → sweep
		if strings.Contains(got, `"mcp__jira__create_issue"`) {
			t.Error("jira entry should be swept from project settings")
		}
	})

}

func TestIntegrationSkillSweepProjectLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create project structure: dir/project/.claude/settings.json
	projectDir := filepath.Join(dir, "project")
	claudeDir := filepath.Join(projectDir, ".claude")

	// Create a skill with SKILL.md
	skillsDir := filepath.Join(claudeDir, "skills", "alive-skill")
	os.MkdirAll(skillsDir, 0o755)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Alive"), 0o644)

	// Create a skill with frontmatter name different from dir name
	fmSkillDir := filepath.Join(claudeDir, "skills", "dir-name")
	os.MkdirAll(fmSkillDir, 0o755)
	os.WriteFile(filepath.Join(fmSkillDir, "SKILL.md"),
		[]byte("---\nname: frontmatter-skill\n---\n# Skill"), 0o644)

	// Create a command .md file
	commandsDir := filepath.Join(claudeDir, "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "alive-cmd.md"), []byte("# Command"), 0o644)

	// Create a command with frontmatter name different from filename
	os.WriteFile(filepath.Join(commandsDir, "file-name.md"),
		[]byte("---\nname: fm-cmd\n---\n# Command"), 0o644)

	// Create a home-level skill (not in project)
	homeSkillDir := filepath.Join(dir, ".claude", "skills", "home-skill")
	os.MkdirAll(homeSkillDir, 0o755)
	os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte("# Home Skill"), 0o644)

	input := `{
  "permissions": {
    "allow": [
      "Skill(alive-skill)",
      "Skill(dead-skill)",
      "Skill(frontmatter-skill)",
      "Skill(dir-name)",
      "Skill(alive-cmd)",
      "Skill(fm-cmd)",
      "Skill(file-name)",
      "Skill(home-skill)",
      "Skill(plugin:my-skill)",
      "Skill(alive-skill *)",
      "Skill(dead-skill *)",
      "Read"
    ],
    "deny": [
      "Skill(denied-dead-skill)"
    ]
  }
}`
	file := filepath.Join(claudeDir, "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	// alive skill should be kept
	if !strings.Contains(got, `"Skill(alive-skill)"`) {
		t.Error("alive Skill(alive-skill) was removed")
	}
	// dead skill should be swept
	if strings.Contains(got, `"Skill(dead-skill)"`) {
		t.Error("dead Skill(dead-skill) was not swept")
	}
	// frontmatter name should be kept
	if !strings.Contains(got, `"Skill(frontmatter-skill)"`) {
		t.Error("frontmatter Skill(frontmatter-skill) was removed")
	}
	// dir name should be swept (frontmatter overrides)
	if strings.Contains(got, `"Skill(dir-name)"`) {
		t.Error("Skill(dir-name) should be swept when frontmatter name differs")
	}
	// alive command should be kept
	if !strings.Contains(got, `"Skill(alive-cmd)"`) {
		t.Error("alive Skill(alive-cmd) was removed")
	}
	// command frontmatter name should be kept
	if !strings.Contains(got, `"Skill(fm-cmd)"`) {
		t.Error("frontmatter Skill(fm-cmd) was removed")
	}
	// command filename should be swept (frontmatter overrides)
	if strings.Contains(got, `"Skill(file-name)"`) {
		t.Error("Skill(file-name) should be swept when frontmatter name differs")
	}
	// home-only skill should be swept in project-level settings
	if strings.Contains(got, `"Skill(home-skill)"`) {
		t.Error("home-only Skill(home-skill) was not swept from project settings")
	}
	// plugin skill should be kept
	if !strings.Contains(got, `"Skill(plugin:my-skill)"`) {
		t.Error("plugin Skill(plugin:my-skill) was removed")
	}
	// prefix match with alive skill should be kept
	if !strings.Contains(got, `"Skill(alive-skill *)"`) {
		t.Error("alive Skill(alive-skill *) was removed")
	}
	// prefix match with dead skill should be swept
	if strings.Contains(got, `"Skill(dead-skill *)"`) {
		t.Error("dead Skill(dead-skill *) was not swept")
	}
	// non-Skill entry should be kept
	if !strings.Contains(got, `"Read"`) {
		t.Error("non-Skill entry was removed")
	}
	// deny entry should never be swept
	if !strings.Contains(got, `"Skill(denied-dead-skill)"`) {
		t.Error("deny Skill entry was incorrectly swept")
	}

	output := buf.String()
	if !strings.Contains(output, "Swept:") {
		t.Errorf("expected swept stats in output: %s", output)
	}
}

func TestIntegrationSkillSweepUserLevel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create home skills and commands directories
	homeSkillDir := filepath.Join(dir, ".claude", "skills", "home-skill")
	os.MkdirAll(homeSkillDir, 0o755)
	os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"), []byte("# Home Skill"), 0o644)

	homeCommandsDir := filepath.Join(dir, ".claude", "commands")
	os.MkdirAll(homeCommandsDir, 0o755)
	os.WriteFile(filepath.Join(homeCommandsDir, "home-cmd.md"), []byte("# Home Cmd"), 0o644)

	// Create a skill with frontmatter name different from dir name
	fmSkillDir := filepath.Join(dir, ".claude", "skills", "fm-dir")
	os.MkdirAll(fmSkillDir, 0o755)
	os.WriteFile(filepath.Join(fmSkillDir, "SKILL.md"),
		[]byte("---\nname: home-fm-skill\n---\n# Skill"), 0o644)

	input := `{
  "permissions": {
    "allow": [
      "Skill(home-skill)",
      "Skill(home-cmd)",
      "Skill(dead-skill)",
      "Skill(plugin:my-skill)",
      "Skill(home-fm-skill)",
      "Skill(fm-dir)"
    ]
  }
}`
	file := filepath.Join(dir, ".claude", "settings.json")
	os.WriteFile(file, []byte(input), 0o644)

	var buf bytes.Buffer
	cli := &CLI{Target: file, Verbose: true, homeDir: dir, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	got := string(data)

	// home skill should be kept
	if !strings.Contains(got, `"Skill(home-skill)"`) {
		t.Error("alive Skill(home-skill) was removed from user settings")
	}
	// home command should be kept
	if !strings.Contains(got, `"Skill(home-cmd)"`) {
		t.Error("alive Skill(home-cmd) was removed from user settings")
	}
	// dead skill should be swept
	if strings.Contains(got, `"Skill(dead-skill)"`) {
		t.Error("dead Skill(dead-skill) was not swept from user settings")
	}
	// plugin skill should be kept
	if !strings.Contains(got, `"Skill(plugin:my-skill)"`) {
		t.Error("plugin Skill(plugin:my-skill) was removed from user settings")
	}
	// frontmatter name skill should be kept
	if !strings.Contains(got, `"Skill(home-fm-skill)"`) {
		t.Error("frontmatter Skill(home-fm-skill) was removed from user settings")
	}
	// dir name should be swept (frontmatter overrides)
	if strings.Contains(got, `"Skill(fm-dir)"`) {
		t.Error("Skill(fm-dir) should be swept from user settings")
	}
}
