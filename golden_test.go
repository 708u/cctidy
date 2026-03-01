package cctidy

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/708u/cctidy/internal/set"
	"github.com/708u/cctidy/internal/testutil"
)

var update = flag.Bool("update", false, "update golden files")

func TestGoldenClaudeJSONAllPathsExist(t *testing.T) {
	t.Parallel()
	input := readTestdata(t, "testdata/golden/claude_json_all_paths_exist/input.json")

	f := NewClaudeJSONFormatter(testutil.AllPathsExist{})
	result, err := f.Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	assertGolden(t, "testdata/golden/claude_json_all_paths_exist/golden.json", result.Data)
}

func TestGoldenClaudeJSONPathCleaning(t *testing.T) {
	t.Parallel()
	input := readTestdata(t, "testdata/golden/claude_json_path_cleaning/input.json")

	checker := testutil.CheckerFor(
		"/Users/test/alive-project",
		"/Users/test/alive-checkout",
	)
	f := NewClaudeJSONFormatter(checker)
	result, err := f.Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	s := result.Stats.(*ClaudeJSONFormatterStats)
	if removed := s.ProjectsBefore - s.ProjectsAfter; removed != 1 {
		t.Errorf("projects removed = %d, want 1", removed)
	}
	if removed := s.RepoBefore - s.RepoAfter; removed != 3 {
		t.Errorf("repo paths removed = %d, want 3", removed)
	}
	if s.RemovedRepos != 1 {
		t.Errorf("removed repos = %d, want 1", s.RemovedRepos)
	}

	assertGolden(t, "testdata/golden/claude_json_path_cleaning/golden.json", result.Data)
}

func TestGoldenSettingsProjectLevel(t *testing.T) {
	t.Parallel()
	input := readTestdata(t, "testdata/golden/settings_project_level/input.json")

	homeDir := filepath.Join(t.TempDir(), "home")
	projectDir := filepath.Join(t.TempDir(), "project")

	// Create agent files in project .claude/agents/
	agentsDir := filepath.Join(projectDir, ".claude", "agents")
	os.MkdirAll(agentsDir, 0o755)
	os.WriteFile(filepath.Join(agentsDir, "stub.md"),
		[]byte("---\nname: stub\n---\n# Stub"), 0o644)

	// Create skill files in project .claude/skills/
	skillsDir := filepath.Join(projectDir, ".claude", "skills", "stub-skill")
	os.MkdirAll(skillsDir, 0o755)
	os.WriteFile(filepath.Join(skillsDir, "SKILL.md"),
		[]byte("# Stub Skill"), 0o644)

	// Create command files in project .claude/commands/
	commandsDir := filepath.Join(projectDir, ".claude", "commands")
	os.MkdirAll(commandsDir, 0o755)
	os.WriteFile(filepath.Join(commandsDir, "stub-cmd.md"),
		[]byte("# Stub Command"), 0o644)

	checker := testutil.CheckerFor(
		"/alive/repo",
		"/alive/data/file.txt",
		filepath.Join(projectDir, "bin/run"),
		filepath.Join(homeDir, "config.json"),
		filepath.Join(homeDir, "alive/notes.md"),
		filepath.Join(projectDir, "src/alive.go"),
		filepath.Join(projectDir, "../alive/output.txt"),
	)
	mcpServers := set.New("github")
	sweeper, err := NewPermissionSweeper(checker, homeDir, mcpServers,
		WithUnsafe(),
		WithProjectLevel(projectDir),
	)
	if err != nil {
		t.Fatalf("NewPermissionSweeper: %v", err)
	}

	result, err := NewSettingsJSONFormatter(sweeper).Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	assertGolden(t, "testdata/golden/settings_project_level/golden.json", result.Data)
}

func TestGoldenSettingsUserLevel(t *testing.T) {
	t.Parallel()
	input := readTestdata(t, "testdata/golden/settings_user_level/input.json")

	homeDir := t.TempDir()

	// Create home agent files
	homeAgentsDir := filepath.Join(homeDir, ".claude", "agents")
	os.MkdirAll(homeAgentsDir, 0o755)
	os.WriteFile(filepath.Join(homeAgentsDir, "home-agent.md"),
		[]byte("---\nname: home-agent\n---\n# Home Agent"), 0o644)

	// Create home skill files
	homeSkillDir := filepath.Join(homeDir, ".claude", "skills", "home-skill")
	os.MkdirAll(homeSkillDir, 0o755)
	os.WriteFile(filepath.Join(homeSkillDir, "SKILL.md"),
		[]byte("# Home Skill"), 0o644)

	// Create home command files
	homeCommandsDir := filepath.Join(homeDir, ".claude", "commands")
	os.MkdirAll(homeCommandsDir, 0o755)
	os.WriteFile(filepath.Join(homeCommandsDir, "home-cmd.md"),
		[]byte("# Home Cmd"), 0o644)

	checker := testutil.CheckerFor(
		"/alive/path",
		filepath.Join(homeDir, "config/settings.json"),
	)
	mcpServers := set.New("github")
	sweeper, err := NewPermissionSweeper(checker, homeDir, mcpServers,
		WithUnsafe(),
	)
	if err != nil {
		t.Fatalf("NewPermissionSweeper: %v", err)
	}

	result, err := NewSettingsJSONFormatter(sweeper).Format(t.Context(), input)
	if err != nil {
		t.Fatalf("format: %v", err)
	}

	s := result.Stats.(*SettingsJSONFormatterStats)
	if s.SweptAllow == 0 {
		t.Error("expected some allow entries to be swept")
	}
	if s.SweptAsk == 0 {
		t.Error("expected some ask entries to be swept")
	}

	assertGolden(t, "testdata/golden/settings_user_level/golden.json", result.Data)
}

func readTestdata(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return data
}

func assertGolden(t *testing.T, goldenPath string, got []byte) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("updating golden: %v", err)
		}
		t.Log("golden file updated")
		return
	}

	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden (run with -update to generate): %v", err)
	}

	if !bytes.Equal(got, golden) {
		t.Errorf("output differs from golden:\n%s", lineDiff(string(golden), string(got)))
	}
}

func lineDiff(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var b strings.Builder
	max := len(wantLines)
	if len(gotLines) > max {
		max = len(gotLines)
	}
	for i := 0; i < max; i++ {
		var w, g string
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if w != g {
			fmt.Fprintf(&b, "line %d:\n  want: %q\n  got:  %q\n", i+1, w, g)
		}
	}
	return b.String()
}
