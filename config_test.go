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
		if cfg.Sweep.Bash.Enabled {
			t.Error("Enabled should be false for missing config")
		}
		if len(cfg.Sweep.Bash.ExcludeEntries) != 0 {
			t.Error("ExcludeEntries should be empty")
		}
	})

	t.Run("empty path uses default and returns zero config", func(t *testing.T) {
		t.Parallel()
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Sweep.Bash.Enabled {
			t.Error("Enabled should be false for default missing config")
		}
	})

	t.Run("full config", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		content := `
[sweep.bash]
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
		if !cfg.Sweep.Bash.Enabled {
			t.Error("Enabled should be true")
		}
		if len(cfg.Sweep.Bash.ExcludeEntries) != 2 {
			t.Errorf("ExcludeEntries len = %d, want 2", len(cfg.Sweep.Bash.ExcludeEntries))
		}
		if len(cfg.Sweep.Bash.ExcludeCommands) != 2 {
			t.Errorf("ExcludeCommands len = %d, want 2", len(cfg.Sweep.Bash.ExcludeCommands))
		}
		if len(cfg.Sweep.Bash.ExcludePaths) != 2 {
			t.Errorf("ExcludePaths len = %d, want 2", len(cfg.Sweep.Bash.ExcludePaths))
		}
	})

	t.Run("enabled false", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte("[sweep.bash]\nenabled = false\n"), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Sweep.Bash.Enabled {
			t.Error("Enabled should be false")
		}
	})

	t.Run("partial config with only commands", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		os.WriteFile(path, []byte("[sweep.bash]\nexclude_commands = [\"mkdir\"]\n"), 0o644)

		cfg, err := LoadConfig(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Sweep.Bash.Enabled {
			t.Error("Enabled should be false when not set")
		}
		if len(cfg.Sweep.Bash.ExcludeCommands) != 1 {
			t.Errorf("ExcludeCommands len = %d, want 1", len(cfg.Sweep.Bash.ExcludeCommands))
		}
		if cfg.Sweep.Bash.ExcludeCommands[0] != "mkdir" {
			t.Errorf("ExcludeCommands[0] = %q, want %q", cfg.Sweep.Bash.ExcludeCommands[0], "mkdir")
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
		if cfg.Sweep.Bash.Enabled {
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
		if got.Sweep.Bash.Enabled != nil {
			t.Error("expected Enabled nil for both zero")
		}
	})

	t.Run("overlay Enabled overrides base", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Sweep.Bash.Enabled = boolPtr(true)
		overlay := rawConfig{}
		overlay.Sweep.Bash.Enabled = boolPtr(false)
		got := mergeRawConfigs(base, overlay)
		if got.Sweep.Bash.Enabled == nil || *got.Sweep.Bash.Enabled {
			t.Error("overlay Enabled=false should win")
		}
	})

	t.Run("overlay Enabled nil preserves base", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Sweep.Bash.Enabled = boolPtr(true)
		got := mergeRawConfigs(base, rawConfig{})
		if got.Sweep.Bash.Enabled == nil || !*got.Sweep.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
	})

	t.Run("arrays union", func(t *testing.T) {
		t.Parallel()
		base := rawConfig{}
		base.Sweep.Bash.ExcludeCommands = []string{"mkdir", "touch"}
		overlay := rawConfig{}
		overlay.Sweep.Bash.ExcludeCommands = []string{"touch", "rm"}
		got := mergeRawConfigs(base, overlay)
		want := []string{"mkdir", "touch", "rm"}
		if !slices.Equal(got.Sweep.Bash.ExcludeCommands, want) {
			t.Errorf("got %v, want %v", got.Sweep.Bash.ExcludeCommands, want)
		}
	})
}

func TestMergeConfig(t *testing.T) {
	t.Parallel()

	t.Run("zero project is no-op", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Sweep.Bash.Enabled = true
		base.Sweep.Bash.ExcludeCommands = []string{"mkdir"}
		got := MergeConfig(base, rawConfig{}, "/project")
		if !got.Sweep.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
		if !slices.Equal(got.Sweep.Bash.ExcludeCommands, []string{"mkdir"}) {
			t.Errorf("commands: got %v, want [mkdir]", got.Sweep.Bash.ExcludeCommands)
		}
	})

	t.Run("nil base treated as zero", func(t *testing.T) {
		t.Parallel()
		project := rawConfig{}
		project.Sweep.Bash.Enabled = boolPtr(true)
		got := MergeConfig(nil, project, "/project")
		if !got.Sweep.Bash.Enabled {
			t.Error("expected Enabled=true from project")
		}
	})

	t.Run("project Enabled overrides base", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Sweep.Bash.Enabled = true
		project := rawConfig{}
		project.Sweep.Bash.Enabled = boolPtr(false)
		got := MergeConfig(base, project, "/project")
		if got.Sweep.Bash.Enabled {
			t.Error("project Enabled=false should override base")
		}
	})

	t.Run("project Enabled nil preserves base", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Sweep.Bash.Enabled = true
		got := MergeConfig(base, rawConfig{}, "/project")
		if !got.Sweep.Bash.Enabled {
			t.Error("base Enabled=true should be preserved")
		}
	})

	t.Run("arrays union with dedup", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Sweep.Bash.ExcludeCommands = []string{"mkdir", "touch"}
		base.Sweep.Bash.ExcludeEntries = []string{"entry1"}
		project := rawConfig{}
		project.Sweep.Bash.ExcludeCommands = []string{"touch", "rm"}
		project.Sweep.Bash.ExcludeEntries = []string{"entry1", "entry2"}
		got := MergeConfig(base, project, "/project")
		wantCmds := []string{"mkdir", "touch", "rm"}
		if !slices.Equal(got.Sweep.Bash.ExcludeCommands, wantCmds) {
			t.Errorf("commands: got %v, want %v", got.Sweep.Bash.ExcludeCommands, wantCmds)
		}
		wantEntries := []string{"entry1", "entry2"}
		if !slices.Equal(got.Sweep.Bash.ExcludeEntries, wantEntries) {
			t.Errorf("entries: got %v, want %v", got.Sweep.Bash.ExcludeEntries, wantEntries)
		}
	})

	t.Run("relative paths resolved against projectRoot", func(t *testing.T) {
		t.Parallel()
		base := &Config{}
		base.Sweep.Bash.ExcludePaths = []string{"/global/path/"}
		project := rawConfig{}
		project.Sweep.Bash.ExcludePaths = []string{"vendor/", "/abs/path/"}
		got := MergeConfig(base, project, "/myproject")
		// filepath.Join strips trailing slash: "vendor/" -> "/myproject/vendor"
		want := []string{"/global/path/", filepath.Join("/myproject", "vendor/"), "/abs/path/"}
		if !slices.Equal(got.Sweep.Bash.ExcludePaths, want) {
			t.Errorf("paths: got %v, want %v", got.Sweep.Bash.ExcludePaths, want)
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
		if got.Sweep.Bash.Enabled != nil {
			t.Error("expected Enabled nil for missing project config")
		}
	})

	t.Run("shared only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[sweep.bash]\nenabled = true\nexclude_commands = [\"mkdir\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sweep.Bash.Enabled == nil || !*got.Sweep.Bash.Enabled {
			t.Error("expected Enabled=true from shared")
		}
		if len(got.Sweep.Bash.ExcludeCommands) != 1 || got.Sweep.Bash.ExcludeCommands[0] != "mkdir" {
			t.Errorf("unexpected ExcludeCommands: %v", got.Sweep.Bash.ExcludeCommands)
		}
	})

	t.Run("local only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("[sweep.bash]\nenabled = false\nexclude_commands = [\"touch\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sweep.Bash.Enabled == nil || *got.Sweep.Bash.Enabled {
			t.Error("expected Enabled=false from local")
		}
	})

	t.Run("local overrides shared", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeDir := filepath.Join(dir, ".claude")
		os.MkdirAll(claudeDir, 0o755)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.toml"),
			[]byte("[sweep.bash]\nenabled = true\nexclude_commands = [\"mkdir\"]\n"), 0o644)
		os.WriteFile(filepath.Join(claudeDir, "cctidy.local.toml"),
			[]byte("[sweep.bash]\nenabled = false\nexclude_commands = [\"touch\"]\n"), 0o644)

		got, err := LoadProjectConfig(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.Sweep.Bash.Enabled == nil || *got.Sweep.Bash.Enabled {
			t.Error("local Enabled=false should override shared")
		}
		wantCmds := []string{"mkdir", "touch"}
		if !slices.Equal(got.Sweep.Bash.ExcludeCommands, wantCmds) {
			t.Errorf("commands: got %v, want %v", got.Sweep.Bash.ExcludeCommands, wantCmds)
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
