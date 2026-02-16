package cctidy

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	toml "github.com/pelletier/go-toml/v2"
)

// Config holds the cctidy configuration loaded from TOML.
type Config struct {
	Permission PermissionConfig `toml:"permission"`
}

// PermissionConfig groups per-tool permission sweep settings.
type PermissionConfig struct {
	// Bash configures sweeping for Bash permission entries.
	// "bash" corresponds to the Bash tool name in Claude Code permissions.
	Bash BashPermissionConfig `toml:"bash"`
}

// BashPermissionConfig controls Bash permission entry sweeping.
type BashPermissionConfig struct {
	// Enabled turns on Bash sweep when true.
	Enabled bool `toml:"enabled"`

	// RemoveCommands lists command names (first token) to always sweep,
	// regardless of path existence. Takes priority over ExcludeCommands.
	RemoveCommands []string `toml:"remove_commands"`

	// ExcludeEntries lists specifiers to exclude by exact match.
	ExcludeEntries []string `toml:"exclude_entries"`

	// ExcludeCommands lists command names (first token) to exclude.
	ExcludeCommands []string `toml:"exclude_commands"`

	// ExcludePaths lists path prefixes to exclude.
	// Trailing / is recommended to ensure directory boundary matching.
	ExcludePaths []string `toml:"exclude_paths"`
}

// rawBashPermissionConfig uses *bool to distinguish "unset" from "false".
type rawBashPermissionConfig struct {
	Enabled         *bool    `toml:"enabled"`
	RemoveCommands  []string `toml:"remove_commands"`
	ExcludeEntries  []string `toml:"exclude_entries"`
	ExcludeCommands []string `toml:"exclude_commands"`
	ExcludePaths    []string `toml:"exclude_paths"`
}

type rawPermissionConfig struct {
	Bash rawBashPermissionConfig `toml:"bash"`
}

type rawConfig struct {
	Permission rawPermissionConfig `toml:"permission"`
}

// defaultConfigPath returns ~/.config/cctidy/config.toml.
func defaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "cctidy", "config.toml"), nil
}

// loadRawConfig reads a TOML file into a rawConfig.
// Returns zero value when the file does not exist.
func loadRawConfig(path string) (rawConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return rawConfig{}, nil
		}
		return rawConfig{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return rawConfig{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return raw, nil
}

// rawToConfig converts a rawConfig to the public Config type.
func rawToConfig(raw rawConfig) *Config {
	cfg := &Config{}
	if raw.Permission.Bash.Enabled != nil {
		cfg.Permission.Bash.Enabled = *raw.Permission.Bash.Enabled
	}
	cfg.Permission.Bash.RemoveCommands = raw.Permission.Bash.RemoveCommands
	cfg.Permission.Bash.ExcludeEntries = raw.Permission.Bash.ExcludeEntries
	cfg.Permission.Bash.ExcludeCommands = raw.Permission.Bash.ExcludeCommands
	cfg.Permission.Bash.ExcludePaths = raw.Permission.Bash.ExcludePaths
	return cfg
}

// unionStrings returns the union of two string slices with duplicates removed.
// Order is preserved: elements from a come first, then new elements from b.
func unionStrings(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	result := slices.Clone(a)
	for _, s := range b {
		if !slices.Contains(result, s) {
			result = append(result, s)
		}
	}
	return result
}

// mergeRawConfigs merges overlay on top of base.
// Enabled: overlay wins if set. Arrays: union with dedup.
func mergeRawConfigs(base, overlay rawConfig) rawConfig {
	merged := rawConfig{}

	// Enabled: overlay wins if explicitly set
	if overlay.Permission.Bash.Enabled != nil {
		merged.Permission.Bash.Enabled = overlay.Permission.Bash.Enabled
	} else {
		merged.Permission.Bash.Enabled = base.Permission.Bash.Enabled
	}

	merged.Permission.Bash.RemoveCommands = unionStrings(
		base.Permission.Bash.RemoveCommands, overlay.Permission.Bash.RemoveCommands)
	merged.Permission.Bash.ExcludeEntries = unionStrings(
		base.Permission.Bash.ExcludeEntries, overlay.Permission.Bash.ExcludeEntries)
	merged.Permission.Bash.ExcludeCommands = unionStrings(
		base.Permission.Bash.ExcludeCommands, overlay.Permission.Bash.ExcludeCommands)
	merged.Permission.Bash.ExcludePaths = unionStrings(
		base.Permission.Bash.ExcludePaths, overlay.Permission.Bash.ExcludePaths)

	return merged
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

	raw, err := loadRawConfig(path)
	if err != nil {
		return nil, err
	}
	return rawToConfig(raw), nil
}

// LoadProjectConfig reads project-level config files from
// <projectRoot>/.claude/cctidy.toml (shared) and
// <projectRoot>/.claude/cctidy.local.toml (local).
// Local overrides shared. Returns zero value when both files are absent.
func LoadProjectConfig(projectRoot string) (rawConfig, error) {
	claudeDir := filepath.Join(projectRoot, ".claude")
	shared, err := loadRawConfig(filepath.Join(claudeDir, "cctidy.toml"))
	if err != nil {
		return rawConfig{}, err
	}
	local, err := loadRawConfig(filepath.Join(claudeDir, "cctidy.local.toml"))
	if err != nil {
		return rawConfig{}, err
	}
	return mergeRawConfigs(shared, local), nil
}

// MergeConfig merges a project rawConfig on top of a global Config.
// Relative paths in the project config's ExcludePaths are resolved
// against projectRoot before merging.
func MergeConfig(base *Config, project rawConfig, projectRoot string) *Config {
	if base == nil {
		base = &Config{}
	}

	merged := &Config{}

	// Enabled: project wins if explicitly set
	if project.Permission.Bash.Enabled != nil {
		merged.Permission.Bash.Enabled = *project.Permission.Bash.Enabled
	} else {
		merged.Permission.Bash.Enabled = base.Permission.Bash.Enabled
	}

	merged.Permission.Bash.RemoveCommands = unionStrings(
		base.Permission.Bash.RemoveCommands, project.Permission.Bash.RemoveCommands)
	merged.Permission.Bash.ExcludeEntries = unionStrings(
		base.Permission.Bash.ExcludeEntries, project.Permission.Bash.ExcludeEntries)
	merged.Permission.Bash.ExcludeCommands = unionStrings(
		base.Permission.Bash.ExcludeCommands, project.Permission.Bash.ExcludeCommands)

	// Resolve relative paths in project config against projectRoot
	resolvedPaths := make([]string, 0, len(project.Permission.Bash.ExcludePaths))
	for _, p := range project.Permission.Bash.ExcludePaths {
		if !filepath.IsAbs(p) {
			p = filepath.Join(projectRoot, p)
		}
		resolvedPaths = append(resolvedPaths, p)
	}
	merged.Permission.Bash.ExcludePaths = unionStrings(
		base.Permission.Bash.ExcludePaths, resolvedPaths)

	return merged
}
