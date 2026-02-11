package cctidy

import (
	"os"
	"path/filepath"
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
