package cctidy

import (
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds the cctidy configuration loaded from TOML.
type Config struct {
	Sweep SweepToolConfig `toml:"sweep"`
}

// SweepToolConfig groups per-tool sweep settings.
type SweepToolConfig struct {
	// Bash configures sweeping for Bash permission entries.
	// "bash" corresponds to the Bash tool name in Claude Code permissions.
	Bash BashSweepConfig `toml:"bash"`
}

// BashSweepConfig controls Bash permission entry sweeping.
type BashSweepConfig struct {
	// Enabled turns on Bash sweep when true.
	Enabled bool `toml:"enabled"`

	// ExcludeEntries lists specifiers to exclude by exact match.
	ExcludeEntries []string `toml:"exclude_entries"`

	// ExcludeCommands lists command names (first token) to exclude.
	ExcludeCommands []string `toml:"exclude_commands"`

	// ExcludePaths lists path prefixes to exclude.
	// Trailing / is recommended to ensure directory boundary matching.
	ExcludePaths []string `toml:"exclude_paths"`
}

// defaultConfigPath returns ~/.config/cctidy/config.toml.
func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cctidy", "config.toml"), nil
}

// LoadConfig reads a TOML configuration file.
// If path is empty, the default path (~/.config/cctidy/config.toml) is used.
// Returns a zero-value Config without error when the file does not exist.
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = defaultConfigPath()
		if err != nil {
			return &Config{}, nil
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return &cfg, nil
}
