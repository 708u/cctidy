package cctidy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/708u/cctidy/internal/set"
)

func TestLoadSkillNames(t *testing.T) {
	t.Parallel()

	t.Run("skill with SKILL.md is registered", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "my-skill")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Skill"), 0o644)

		s := LoadSkillNames(claudeDir)
		if !s.Has("my-skill") {
			t.Error("skill name should be in set")
		}
	})

	t.Run("command .md file is registered", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(filepath.Join(commandsDir, "review.md"), []byte("# Review"), 0o644)

		s := LoadSkillNames(claudeDir)
		if !s.Has("review") {
			t.Error("command name should be in set")
		}
	})

	t.Run("same name in skills and commands is unified", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "deploy")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(skillsDir, 0o755)
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Skill"), 0o644)
		os.WriteFile(filepath.Join(commandsDir, "deploy.md"), []byte("# Command"), 0o644)

		s := LoadSkillNames(claudeDir)
		if !s.Has("deploy") {
			t.Error("unified name should be in set")
		}
	})

	t.Run("skills only without commands dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "only-skill")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Skill"), 0o644)

		s := LoadSkillNames(claudeDir)
		if !s.Has("only-skill") {
			t.Error("skill name should be in set")
		}
	})

	t.Run("commands only without skills dir", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(filepath.Join(commandsDir, "only-cmd.md"), []byte("# Cmd"), 0o644)

		s := LoadSkillNames(claudeDir)
		if !s.Has("only-cmd") {
			t.Error("command name should be in set")
		}
	})

	t.Run("non-existent directory returns empty set", func(t *testing.T) {
		t.Parallel()
		s := LoadSkillNames("/nonexistent/dir")
		if s.Len() != 0 {
			t.Errorf("expected empty set, got %v", s)
		}
	})

	t.Run("empty string returns empty set", func(t *testing.T) {
		t.Parallel()
		s := LoadSkillNames("")
		if s.Len() != 0 {
			t.Errorf("expected empty set, got %v", s)
		}
	})

	t.Run("skill dir without SKILL.md is ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "no-skill-md")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(filepath.Join(skillsDir, "README.md"), []byte("# Not a skill"), 0o644)

		s := LoadSkillNames(claudeDir)
		if s.Has("no-skill-md") {
			t.Error("dir without SKILL.md should not be in set")
		}
	})

	t.Run("non .md file in commands is ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(filepath.Join(commandsDir, "readme.txt"), []byte("not a command"), 0o644)
		os.WriteFile(filepath.Join(commandsDir, "valid.md"), []byte("# Valid"), 0o644)

		s := LoadSkillNames(claudeDir)
		if s.Has("readme") {
			t.Error("non-.md file should not be in set")
		}
		if !s.Has("valid") {
			t.Error("valid .md file should be in set")
		}
	})

	t.Run("subdirectory in commands is ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.Mkdir(filepath.Join(commandsDir, "subdir.md"), 0o755)

		s := LoadSkillNames(claudeDir)
		if s.Has("subdir") {
			t.Error("directory should not be in set")
		}
	})

	t.Run("skill frontmatter name overrides dir name", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "dir-name")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(
			filepath.Join(skillsDir, "SKILL.md"),
			[]byte("---\nname: custom-name\n---\n# Skill"),
			0o644,
		)

		s := LoadSkillNames(claudeDir)
		if !s.Has("custom-name") {
			t.Error("frontmatter name should be in set")
		}
		if s.Has("dir-name") {
			t.Error("dir name should not be in set when frontmatter name exists")
		}
	})

	t.Run("skill without frontmatter name uses dir name", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills", "fallback-skill")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(
			filepath.Join(skillsDir, "SKILL.md"),
			[]byte("---\ndescription: no name field\n---\n# Skill"),
			0o644,
		)

		s := LoadSkillNames(claudeDir)
		if !s.Has("fallback-skill") {
			t.Error("dir name should be used when no frontmatter name")
		}
	})

	t.Run("command frontmatter name overrides filename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(
			filepath.Join(commandsDir, "file-name.md"),
			[]byte("---\nname: custom-cmd\ndescription: test\n---\n# Cmd"),
			0o644,
		)

		s := LoadSkillNames(claudeDir)
		if !s.Has("custom-cmd") {
			t.Error("frontmatter name should be in set")
		}
		if s.Has("file-name") {
			t.Error("filename should not be in set when frontmatter name exists")
		}
	})

	t.Run("command without frontmatter name uses filename", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		commandsDir := filepath.Join(claudeDir, "commands")
		os.MkdirAll(commandsDir, 0o755)
		os.WriteFile(
			filepath.Join(commandsDir, "fallback-cmd.md"),
			[]byte("---\ndescription: no name\n---\n# Cmd"),
			0o644,
		)

		s := LoadSkillNames(claudeDir)
		if !s.Has("fallback-cmd") {
			t.Error("filename should be used when no frontmatter name")
		}
	})

	t.Run("file in skills dir is ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		skillsDir := filepath.Join(claudeDir, "skills")
		os.MkdirAll(skillsDir, 0o755)
		os.WriteFile(filepath.Join(skillsDir, "stray-file.md"), []byte("# Stray"), 0o644)

		s := LoadSkillNames(claudeDir)
		if s.Has("stray-file") {
			t.Error("file directly in skills dir should not be in set")
		}
	})
}

func TestSkillToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()

	skillsWithReview := set.New("review")
	skillsWithDeploy := set.New("deploy")
	emptySkills := set.New[string]()

	tests := []struct {
		name      string
		sweeper   *SkillToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "plugin skill with colon is kept",
			sweeper:   NewSkillToolSweeper(emptySkills),
			specifier: "plugin:skill-name",
			wantSweep: false,
		},
		{
			name:      "skill in set is kept",
			sweeper:   NewSkillToolSweeper(skillsWithReview),
			specifier: "review",
			wantSweep: false,
		},
		{
			name:      "command in set is kept",
			sweeper:   NewSkillToolSweeper(skillsWithDeploy),
			specifier: "deploy",
			wantSweep: false,
		},
		{
			name:      "skill not in set is swept",
			sweeper:   NewSkillToolSweeper(skillsWithReview),
			specifier: "dead-skill",
			wantSweep: true,
		},
		{
			name:      "prefix match extracts name",
			sweeper:   NewSkillToolSweeper(skillsWithReview),
			specifier: "review *",
			wantSweep: false,
		},
		{
			name:      "prefix match with dead name is swept",
			sweeper:   NewSkillToolSweeper(skillsWithReview),
			specifier: "dead-skill *",
			wantSweep: true,
		},
		{
			name:      "empty set keeps conservatively",
			sweeper:   NewSkillToolSweeper(emptySkills),
			specifier: "unknown",
			wantSweep: false,
		},
		{
			name:      "nil set keeps conservatively",
			sweeper:   NewSkillToolSweeper(nil),
			specifier: "unknown",
			wantSweep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			entry := StandardEntry{Tool: ToolSkill, Specifier: tt.specifier}
			result := tt.sweeper.ShouldSweep(t.Context(), entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}
