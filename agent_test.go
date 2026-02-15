package cctidy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAgentNames(t *testing.T) {
	t.Parallel()

	t.Run("only frontmatter name is registered", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, "agents")
		os.Mkdir(agentsDir, 0o755)

		os.WriteFile(
			filepath.Join(agentsDir, "frontmatter-agent.md"),
			[]byte("---\nname: custom-name\n---\n# Agent\n"),
			0o644,
		)

		names := LoadAgentNames(agentsDir)

		if !names.Has("custom-name") {
			t.Error("frontmatter name should be in set")
		}
		if names.Has("frontmatter-agent") {
			t.Error("filename should not be in set when frontmatter name exists")
		}
	})

	t.Run("non-existent directory returns empty set", func(t *testing.T) {
		t.Parallel()
		names := LoadAgentNames("/nonexistent/dir")
		if names.Len() != 0 {
			t.Errorf("expected empty set, got %v", names)
		}
	})

	t.Run("empty dir string returns empty set", func(t *testing.T) {
		t.Parallel()
		names := LoadAgentNames("")
		if names.Len() != 0 {
			t.Errorf("expected empty set, got %v", names)
		}
	})

	t.Run("non-md files and missing frontmatter are ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, "agents")
		os.Mkdir(agentsDir, 0o755)

		os.WriteFile(
			filepath.Join(agentsDir, "readme.txt"),
			[]byte("not an agent"),
			0o644,
		)
		os.WriteFile(
			filepath.Join(agentsDir, "no-frontmatter.md"),
			[]byte("# Agent without frontmatter\n"),
			0o644,
		)
		os.WriteFile(
			filepath.Join(agentsDir, "valid.md"),
			[]byte("---\nname: valid-agent\n---\n"),
			0o644,
		)

		names := LoadAgentNames(agentsDir)
		if names.Has("readme") {
			t.Error("non-.md file should not be in set")
		}
		if names.Has("no-frontmatter") {
			t.Error("file without frontmatter name should not be in set")
		}
		if !names.Has("valid-agent") {
			t.Error("valid frontmatter name should be in set")
		}
	})

	t.Run("directories are ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, "agents")
		os.Mkdir(agentsDir, 0o755)
		os.Mkdir(filepath.Join(agentsDir, "subdir.md"), 0o755)

		names := LoadAgentNames(agentsDir)
		if names.Has("subdir") {
			t.Error("directory should not be in set")
		}
	})
}
