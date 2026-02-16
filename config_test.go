package cctidy

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	t.Run("file not found returns zero config", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadConfig("/nonexistent/config.toml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be false for missing config")
		}
		if len(cfg.Permission.Bash.ExcludeEntries) != 0 {
			t.Error("ExcludeEntries should be empty")
		}
	})

	t.Run("empty path uses default and returns zero config", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be false for default missing config")
		}
	})

	t.Run("full config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[permission.bash]
enabled = true
exclude_entries = [
  "mkdir -p /opt/myapp/logs",
  "touch /opt/myapp/.initialized",
]
exclude_commands = [
  "mkdir",
  "touch",
]
exclude_paths = [
  "/opt/myapp/",
  "/var/log/myapp/",
]
`
		os.WriteFile(path, []byte(content), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be true")
		}
		if len(cfg.Permission.Bash.ExcludeEntries) != 2 {
			t.Errorf("ExcludeEntries len = %d, want 2", len(cfg.Permission.Bash.ExcludeEntries))
		}
		if len(cfg.Permission.Bash.ExcludeCommands) != 2 {
			t.Errorf("ExcludeCommands len = %d, want 2", len(cfg.Permission.Bash.ExcludeCommands))
		}
		if len(cfg.Permission.Bash.ExcludePaths) != 2 {
			t.Errorf("ExcludePaths len = %d, want 2", len(cfg.Permission.Bash.ExcludePaths))
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte("[permission.bash]\nenabled = false\n"), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be false")
		}
	})

	t.Run("partial config with only commands", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte("[permission.bash]\nexclude_commands = [\"mkdir\"]\n"), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be false when not set")
		}
		if len(cfg.Permission.Bash.ExcludeCommands) != 1 {
			t.Errorf("ExcludeCommands len = %d, want 1", len(cfg.Permission.Bash.ExcludeCommands))
		}
		if cfg.Permission.Bash.ExcludeCommands[0] != "mkdir" {
			t.Errorf("ExcludeCommands[0] = %q, want %q", cfg.Permission.Bash.ExcludeCommands[0], "mkdir")
		}
	})

	t.Run("invalid TOML returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte("invalid[[[toml"), 0o644)

		_, err := LoadConfig(path)
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
	})

	t.Run("empty file returns zero config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte(""), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Permission.Bash.Enabled {
			t.Error("Enabled should be false for empty config")
		}
	})
}

func boolPtr(v bool) *bool { return &v }

func TestUnionStrings(t *testing.T) {
	t.Parallel()

	t.Run("both empty", func(t *testing.T) {
		t.Parallel()
		got := unionStrings(nil, nil)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		t.Parallel()
		got := unionStrings([]string{"a", "b"}, []string{"c", "d"})
		want := []string{"a", "b", "c", "d"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("with overlap", func(t *testing.T) {
		t.Parallel()
		got := unionStrings([]string{"a", "b"}, []string{"b", "c"})
		want := []string{"a", "b", "c"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("first empty", func(t *testing.T) {
		t.Parallel()
		got := unionStrings(nil, []string{"a"})
		want := []string{"a"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("second empty", func(t *testing.T) {
		t.Parallel()
		got := unionStrings([]string{"a"}, nil)
		want := []string{"a"}
		if !slices.Equal(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func TestMergeRawConfigs(t *testing.T) {
	t.Parallel()

	t.Run("both zero", func(t *testing.T) {
		t.Parallel()
		got := mergeRawConfigs(rawConfig{}, rawConfig{})
		if got.Permission.Bash.Enabled != nil {
			t.Error("expected Enabled nil for both zero")
		}
	})

	t.Run("overlay Enabled overrides base", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Permission.Bash.Enabled = boolPtr(true)
		overlay := rawConfig{}
		overlay.Permission.Bash.Enabled = boolPtr(false)
		got := mergeRawConfigs(base, overlay)
		if got.Permission.Bash.Enabled == nil || *got.Permission.Bash.Enabled {
			t.Error("overlay Enabled=false should win")
		}
	})

	t.Run("overlay Enabled nil preserves base", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Permission.Bash.Enabled = boolPtr(true)
		got := mergeRawConfigs(base, rawConfig{})
		if got.Permission.Bash.Enabled == nil || !*got.Permission.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
	})

	t.Run("arrays union", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Permission.Bash.ExcludeCommands = []string{"mkdir", "touch"}
		overlay := rawConfig{}
		overlay.Permission.Bash.ExcludeCommands = []string{"touch", "rm"}
		got := mergeRawConfigs(base, overlay)
		want := []string{"mkdir", "touch", "rm"}
		if !slices.Equal(got.Permission.Bash.ExcludeCommands, want) {
			t.Errorf("got %v, want %v", got.Permission.Bash.ExcludeCommands, want)
		}
	})

	t.Run("RemoveCommands union", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Permission.Bash.RemoveCommands = []string{"npm", "pip"}
		overlay := rawConfig{}
		overlay.Permission.Bash.RemoveCommands = []string{"pip", "yarn"}
		got := mergeRawConfigs(base, overlay)
		want := []string{"npm", "pip", "yarn"}
		if !slices.Equal(got.Permission.Bash.RemoveCommands, want) {
			t.Errorf("got %v, want %v", got.Permission.Bash.RemoveCommands, want)
		}
	})
}

func TestMergeConfig(t *testing.T) {
	t.Parallel()

	t.Run("zero project is no-op", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.Enabled = true
		base.Permission.Bash.ExcludeCommands = []string{"mkdir"}
		got := MergeConfig(base, rawConfig{}, "/project")
		if !got.Permission.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
		if !slices.Equal(got.Permission.Bash.ExcludeCommands, []string{"mkdir"}) {
			t.Errorf("commands: got %v, want [mkdir]", got.Permission.Bash.ExcludeCommands)
		}
	})

	t.Run("nil base treated as zero", func(t *testing.T) {
		t.Parallel()
		project := rawConfig{}
		project.Permission.Bash.Enabled = boolPtr(true)
		got := MergeConfig(nil, project, "/project")
		if !got.Permission.Bash.Enabled {
			t.Error("expected Enabled=true from project")
		}
	})

	t.Run("project Enabled overrides base", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.Enabled = true
		project := rawConfig{}
		project.Permission.Bash.Enabled = boolPtr(false)
		got := MergeConfig(base, project, "/project")
		if got.Permission.Bash.Enabled {
			t.Error("project Enabled=false should override base")
		}
	})

	t.Run("project Enabled nil preserves base", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.Enabled = true
		got := MergeConfig(base, rawConfig{}, "/project")
		if !got.Permission.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
	})

	t.Run("arrays union with dedup", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.ExcludeCommands = []string{"mkdir", "touch"}
		base.Permission.Bash.ExcludeEntries = []string{"entry1"}
		project := rawConfig{}
		project.Permission.Bash.ExcludeCommands = []string{"touch", "rm"}
		project.Permission.Bash.ExcludeEntries = []string{"entry1", "entry2"}
		got := MergeConfig(base, project, "/project")
		wantCmds := []string{"mkdir", "touch", "rm"}
		if !slices.Equal(got.Permission.Bash.ExcludeCommands, wantCmds) {
			t.Errorf("commands: got %v, want %v", got.Permission.Bash.ExcludeCommands, wantCmds)
		}
		wantEntries := []string{"entry1", "entry2"}
		if !slices.Equal(got.Permission.Bash.ExcludeEntries, wantEntries) {
			t.Errorf("entries: got %v, want %v", got.Permission.Bash.ExcludeEntries, wantEntries)
		}
	})

	t.Run("RemoveCommands union with dedup", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.RemoveCommands = []string{"npm", "pip"}
		project := rawConfig{}
		project.Permission.Bash.RemoveCommands = []string{"pip", "yarn"}
		got := MergeConfig(base, project, "/project")
		want := []string{"npm", "pip", "yarn"}
		if !slices.Equal(got.Permission.Bash.RemoveCommands, want) {
			t.Errorf("RemoveCommands: got %v, want %v", got.Permission.Bash.RemoveCommands, want)
		}
	})

	t.Run("relative paths resolved against projectRoot", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Permission.Bash.ExcludePaths = []string{"/global/path/"}
		project := rawConfig{}
		project.Permission.Bash.ExcludePaths = []string{"vendor/", "/abs/path/"}
		got := MergeConfig(base, project, "/myproject")
		// filepath.Join strips trailing slash: "vendor/" -> "/myproject/vendor"
		want := []string{"/global/path/", filepath.Join("/myproject", "vendor/"), "/abs/path/"}
		if !slices.Equal(got.Permission.Bash.ExcludePaths, want) {
			t.Errorf("paths: got %v, want %v", got.Permission.Bash.ExcludePaths, want)
		}
	})
}

func TestLoadProjectConfig(t *testing.T) {
	t.Parallel()

	t.Run("both absent returns zero", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Permission.Bash.Enabled != nil {
			t.Error("expected Enabled nil for missing project config")
		}
	})

	t.Run("shared only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[permission.bash]\nenabled = true\nexclude_commands = [\"mkdir\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Permission.Bash.Enabled == nil || !*got.Permission.Bash.Enabled {
			t.Error("expected Enabled=true from shared")
		}
		if len(got.Permission.Bash.ExcludeCommands) != 1 || got.Permission.Bash.ExcludeCommands[0] != "mkdir" {
			t.Errorf("unexpected ExcludeCommands: %v", got.Permission.Bash.ExcludeCommands)
		}
	})

	t.Run("local only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("[permission.bash]\nenabled = false\nexclude_commands = [\"touch\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Permission.Bash.Enabled == nil || *got.Permission.Bash.Enabled {
			t.Error("expected Enabled=false from local")
		}
	})

	t.Run("local overrides shared", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[permission.bash]\nenabled = true\nexclude_commands = [\"mkdir\"]\n"), 0o644)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("[permission.bash]\nenabled = false\nexclude_commands = [\"touch\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Permission.Bash.Enabled == nil || *got.Permission.Bash.Enabled {
			t.Error("local Enabled=false should override shared")
		}
		wantCmds := []string{"mkdir", "touch"}
		if !slices.Equal(got.Permission.Bash.ExcludeCommands, wantCmds) {
			t.Errorf("commands: got %v, want %v", got.Permission.Bash.ExcludeCommands, wantCmds)
		}
	})

	t.Run("invalid shared TOML returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("invalid[[[toml"), 0o644)

		_, err := LoadProjectConfig(dir)
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
	})

	t.Run("invalid local TOML returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("invalid[[[toml"), 0o644)

		_, err := LoadProjectConfig(dir)
		if err == nil {
			t.Fatal("expected error for invalid TOML")
		}
	})
}
