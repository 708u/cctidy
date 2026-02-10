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
)

type alwaysTrue struct{}

func (alwaysTrue) Exists(string) bool { return true }

var update = flag.Bool("update", false, "update golden files")

func TestGolden(t *testing.T) {
	t.Parallel()
	input, err := os.ReadFile("testdata/input.json")
	if err != nil {
		t.Fatalf("reading input: %v", err)
	}

	f := cctidy.NewClaudeJSONFormatter(alwaysTrue{})
	result, err := f.Format(input)
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

	result, err := cctidy.NewSettingsJSONFormatter().Format(input)
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
	if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: file, Verbose: true, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: file, DryRun: true, Verbose: true, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: file, Backup: true, Verbose: true, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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

	t.Run("preserves file permissions", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, ".claude.json")
		os.WriteFile(file, []byte(input), 0o600)

		var buf bytes.Buffer
		cli := &CLI{Target: file, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: "/nonexistent/path/test.json", checker: alwaysTrue{}, w: &buf}
		err := cli.Run("/tmp")
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
		cli := &CLI{Verbose: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(alwaysTrue{})},
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		if err := cli.runTargets(targets); err != nil {
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
		cli := &CLI{Verbose: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: claudeJSON, formatter: cctidy.NewClaudeJSONFormatter(alwaysTrue{})},
			{path: missingFile, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		if err := cli.runTargets(targets); err != nil {
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
		cli := &CLI{Verbose: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: settingsJSON, formatter: cctidy.NewSettingsJSONFormatter()},
			{path: filepath.Join(dir, "missing.json"), formatter: cctidy.NewSettingsJSONFormatter()},
		}
		if err := cli.runTargets(targets); err != nil {
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
		cli := &CLI{Target: settingsJSON, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: file, Check: true, checker: alwaysTrue{}, w: &buf}
		if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: file, Check: true, checker: alwaysTrue{}, w: &buf}
		err := cli.Run(dir)
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
		cli := &CLI{Target: file, Check: true, checker: alwaysTrue{}, w: &buf}
		cli.Run(dir)

		data, _ := os.ReadFile(file)
		if string(data) != input {
			t.Errorf("file was modified in check mode")
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		cli := &CLI{Target: "/nonexistent/path/test.json", Check: true, checker: alwaysTrue{}, w: &buf}
		err := cli.Run("/tmp")
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
		cli := &CLI{Target: file, Check: true, checker: alwaysTrue{}, w: &buf}
		err := cli.Run(dir)
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
		cli := &CLI{Check: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter()},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		if err := cli.runTargets(targets); err != nil {
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
		cli := &CLI{Check: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter()},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		err := cli.runTargets(targets)
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
		cli := &CLI{Check: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter()},
			{path: missing, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		if err := cli.runTargets(targets); err != nil {
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
		cli := &CLI{Check: true, Verbose: true, checker: alwaysTrue{}, w: &buf}
		targets := []targetFile{
			{path: f1, formatter: cctidy.NewSettingsJSONFormatter()},
			{path: f2, formatter: cctidy.NewSettingsJSONFormatter()},
		}
		cli.runTargets(targets)

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
	cli := &CLI{Target: file, checker: alwaysTrue{}, w: &buf}
	if err := cli.Run(dir); err != nil {
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
		cli := &CLI{Target: "/some/path.json", checker: alwaysTrue{}}
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
		cli := &CLI{Target: "/home/user/.claude.json", checker: alwaysTrue{}}
		targets := cli.resolveTargets("/home/user")
		if _, ok := targets[0].formatter.(*cctidy.ClaudeJSONFormatter); !ok {
			t.Errorf("claude.json should use *cctidy.ClaudeJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("settings.json target uses SettingsJSONFormatter", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{Target: "/home/user/.claude/settings.json", checker: alwaysTrue{}}
		targets := cli.resolveTargets("/home/user")
		if _, ok := targets[0].formatter.(*cctidy.SettingsJSONFormatter); !ok {
			t.Errorf("settings.json should use *cctidy.SettingsJSONFormatter, got %T", targets[0].formatter)
		}
	})

	t.Run("without target returns default targets", func(t *testing.T) {
		t.Parallel()
		cli := &CLI{checker: alwaysTrue{}}
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
