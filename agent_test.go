package cctidy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAgentName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "name field present",
			data: "---\nname: my-agent\n---\n# Agent\n",
			want: "my-agent",
		},
		{
			name: "name field with other fields",
			data: "---\nname: custom\ndescription: test\n---\n",
			want: "custom",
		},
		{
			name: "no name field",
			data: "---\ndescription: test\n---\n# Agent\n",
			want: "",
		},
		{
			name: "no frontmatter",
			data: "# Agent\nSome content\n",
			want: "",
		},
		{
			name: "empty input",
			data: "",
			want: "",
		},
		{
			name: "invalid YAML frontmatter",
			data: "---\n: invalid: yaml:\n---\n",
			want: "",
		},
		{
			name: "name is not a string",
			data: "---\nname: 123\n---\n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := parseAgentName([]byte(tt.data))
			if got != tt.want {
				t.Errorf("parseAgentName() = %q, want %q", got, tt.want)
			}
		})
	}
}

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

		set := LoadAgentNames(agentsDir)

		if !set["custom-name"] {
			t.Error("frontmatter name should be in set")
		}
		if set["frontmatter-agent"] {
			t.Error("filename should not be in set when frontmatter name exists")
		}
	})

	t.Run("non-existent directory returns empty set", func(t *testing.T) {
		t.Parallel()
		set := LoadAgentNames("/nonexistent/dir")
		if len(set) != 0 {
			t.Errorf("expected empty set, got %v", set)
		}
	})

	t.Run("empty dir string returns empty set", func(t *testing.T) {
		t.Parallel()
		set := LoadAgentNames("")
		if len(set) != 0 {
			t.Errorf("expected empty set, got %v", set)
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

		set := LoadAgentNames(agentsDir)
		if set["readme"] {
			t.Error("non-.md file should not be in set")
		}
		if set["no-frontmatter"] {
			t.Error("file without frontmatter name should not be in set")
		}
		if !set["valid-agent"] {
			t.Error("valid frontmatter name should be in set")
		}
	})

	t.Run("directories are ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		agentsDir := filepath.Join(dir, "agents")
		os.Mkdir(agentsDir, 0o755)
		os.Mkdir(filepath.Join(agentsDir, "subdir.md"), 0o755)

		set := LoadAgentNames(agentsDir)
		if set["subdir"] {
			t.Error("directory should not be in set")
		}
	})
}
