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
	checker := testutil.CheckerFor(
		"/alive/repo",
		"/alive/data/file.txt",
		filepath.Join(baseDir, "bin/run"),
		filepath.Join(homeDir, "config.json"),
		filepath.Join(homeDir, "alive/notes.md"),
		filepath.Join(baseDir, "src/alive.go"),
		filepath.Join(baseDir, "../alive/output.txt"),
	)
	sweeper := cctidy.NewPermissionSweeper(checker, homeDir, cctidy.WithBashSweep(cctidy.BashSweepConfig{}), cctidy.WithBaseDir(baseDir))
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
	cli := &CLI{Target: file, Verbose: true, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, DryRun: true, Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, Backup: true, Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: "/nonexistent/path/test.json", checker: testutil.AllPathsExist{}, w: &buf}
		err := cli.Run(t.Context(), "/tmp")
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
		cli := &CLI{Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(testutil.AllPathsExist{})},
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(testutil.AllPathsExist{})},
			{path: missingFile, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
			{path: filepath.Join(dir, "missing.json"), formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Target: settingsJSON, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		err := cli.Run(t.Context(), dir)
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
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		cli.Run(t.Context(), dir)

		data, _ := os.ReadFile(file)
		if string(data) != input {
			t.Errorf("file was modified in check mode")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cli := &CLI{Target: "/nonexistent/path/test.json", Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		err := cli.Run(t.Context(), "/tmp")
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
		cli := &CLI{Target: file, Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		err := cli.Run(t.Context(), dir)
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
		cli := &CLI{Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Check: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
			{path: missing, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
		cli := &CLI{Check: true, Verbose: true, checker: testutil.AllPathsExist{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter(cctidy.NewPermissionSweeper(testutil.AllPathsExist{}, ""))},
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
	cli := &CLI{Target: file, checker: testutil.AllPathsExist{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
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
		cli := &CLI{Target: "/some/path.json", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets("/home/user")
		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}
		if targets[0].path != "/some/path.json" {
			t.Errorf("unexpected path: %s", targets[0].path)
		}
	})

	t.Run("claude.json target uses ClaudeJSONFormatter", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/home/user/.claude.json", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets("/home/user")
		if _, ok := targets[0].formatter.(*cctidy.ClaudeJSONFormatter); !ok {
			t.Errorf("claude.json should use *cctidy.ClaudeJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("settings.json target uses SettingsJSONFormatter", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/home/user/.claude/settings.json", checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets("/home/user")
		if _, ok := targets[0].formatter.(*cctidy.SettingsJSONFormatter); !ok {
			t.Errorf("settings.json should use *cctidy.SettingsJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("without target returns default targets", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{checker: testutil.AllPathsExist{}}
		targets := cli.resolveTargets("/home/user")
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
	cli := &CLI{Target: file, Verbose: true, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
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
	cli := &CLI{Target: file, IncludeBashTool: true, Verbose: true, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
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
	cli := &CLI{Target: file, Verbose: true, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
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
	cli := &CLI{Target: file, Check: true, checker: &osPathChecker{}, w: &buf}
	err := cli.Run(t.Context(), dir)
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
			Target:          file,
			IncludeBashTool: true,
			Verbose:         true,
			checker:         &osPathChecker{},
			cfg:             cfg,
			w:               &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
			Target:          file,
			IncludeBashTool: true,
			Verbose:         true,
			checker:         &osPathChecker{},
			cfg:             cfg,
			w:               &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
			Target:          file,
			IncludeBashTool: true,
			Verbose:         true,
			checker:         &osPathChecker{},
			cfg:             cfg,
			w:               &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
			Target:          file,
			IncludeBashTool: true,
			Verbose:         true,
			checker:         &osPathChecker{},
			cfg:             cfg,
			w:               &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, _ := os.ReadFile(file)
		got := string(data)

		if strings.Contains(got, `"Bash(git -C `+deadPath) {
			t.Error("CLI --include-bash-tool should override config enabled=false")
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
			checker: &osPathChecker{},
			cfg:     cfg,
			w:       &buf,
		}
		if err := cli.Run(t.Context(), dir); err != nil {
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
	cli := &CLI{Target: file, DryRun: true, Verbose: true, checker: &osPathChecker{}, w: &buf}
	if err := cli.Run(t.Context(), dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != input {
		t.Error("file was modified in dry-run mode")
	}
}
