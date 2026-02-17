package cctidy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractPluginNameFromKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key  string
		want string
	}{
		{"github@claude-plugins-official", "github"},
		{"linter@acme-tools", "linter"},
		{"formatter@acme-tools", "formatter"},
		{"@marketplace", ""},
		{"no-at-sign", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			if got := extractPluginNameFromKey(tt.key); got != tt.want {
				t.Errorf("extractPluginNameFromKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestExtractPluginNameFromEntry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		entry  string
		want   string
		wantOK bool
	}{
		{
			name:   "MCP plugin entry",
			entry:  "mcp__plugin_github_github__search_code",
			want:   "github",
			wantOK: true,
		},
		{
			name:   "MCP plugin entry with parens",
			entry:  "mcp__plugin_github_github__search_code(query)",
			want:   "github",
			wantOK: true,
		},
		{
			name:   "MCP plugin bare server",
			entry:  "mcp__plugin_linter_acme",
			want:   "linter",
			wantOK: true,
		},
		{
			name:   "MCP plugin entry with parens bare",
			entry:  "mcp__plugin_linter_acme__check(args)",
			want:   "linter",
			wantOK: true,
		},
		{
			name:   "Skill plugin entry",
			entry:  "Skill(linter:lint-check)",
			want:   "linter",
			wantOK: true,
		},
		{
			name:   "Skill plugin entry with suffix",
			entry:  "Skill(github:review *)",
			want:   "github",
			wantOK: true,
		},
		{
			name:   "Task plugin entry",
			entry:  "Task(linter:lint-agent)",
			want:   "linter",
			wantOK: true,
		},
		{
			name:   "non-plugin MCP entry",
			entry:  "mcp__slack__post_message",
			want:   "",
			wantOK: false,
		},
		{
			name:   "non-plugin Skill entry",
			entry:  "Skill(review)",
			want:   "",
			wantOK: false,
		},
		{
			name:   "non-plugin Task entry",
			entry:  "Task(Explore)",
			want:   "",
			wantOK: false,
		},
		{
			name:   "Read entry",
			entry:  "Read(/some/path)",
			want:   "",
			wantOK: false,
		},
		{
			name:   "bare Read",
			entry:  "Read",
			want:   "",
			wantOK: false,
		},
		{
			name:   "empty string",
			entry:  "",
			want:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := extractPluginNameFromEntry(tt.entry)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("extractPluginNameFromEntry(%q) = (%q, %v), want (%q, %v)",
					tt.entry, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestExtractMCPPluginName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		entry  string
		want   string
		wantOK bool
	}{
		{
			name:   "standard mcp plugin",
			entry:  "mcp__plugin_github_github__search_code",
			want:   "github",
			wantOK: true,
		},
		{
			name:   "bare mcp plugin",
			entry:  "mcp__plugin_linter_acme",
			want:   "linter",
			wantOK: true,
		},
		{
			name:   "with parens",
			entry:  "mcp__plugin_formatter_tools__format(args)",
			want:   "formatter",
			wantOK: true,
		},
		{
			name:   "empty after prefix",
			entry:  "mcp__plugin_",
			want:   "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := extractMCPPluginName(tt.entry)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("extractMCPPluginName(%q) = (%q, %v), want (%q, %v)",
					tt.entry, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestLoadEnabledPlugins(t *testing.T) {
	t.Parallel()

	t.Run("single file with plugins", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(`{
			"enabledPlugins": {
				"github@claude-plugins-official": true,
				"linter@acme-tools": false
			}
		}`), 0o644)

		ep := LoadEnabledPlugins(file)
		if ep == nil {
			t.Fatal("expected non-nil EnabledPlugins")
		}
		if !ep.IsEnabled("github") {
			t.Error("github should be enabled")
		}
		if ep.IsEnabled("linter") {
			t.Error("linter should be disabled")
		}
	})

	t.Run("no files exist returns nil", func(t *testing.T) {
		t.Parallel()
		ep := LoadEnabledPlugins("/nonexistent/a.json", "/nonexistent/b.json")
		if ep != nil {
			t.Error("expected nil for missing files")
		}
	})

	t.Run("files without enabledPlugins returns nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "settings.json")
		os.WriteFile(file, []byte(`{"permissions": {}}`), 0o644)

		ep := LoadEnabledPlugins(file)
		if ep != nil {
			t.Error("expected nil when no enabledPlugins key")
		}
	})

	t.Run("multiple files OR merged", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		f1 := filepath.Join(dir, "a.json")
		os.WriteFile(f1, []byte(`{
			"enabledPlugins": {
				"github@official": false,
				"linter@acme": true
			}
		}`), 0o644)
		f2 := filepath.Join(dir, "b.json")
		os.WriteFile(f2, []byte(`{
			"enabledPlugins": {
				"github@other": true,
				"formatter@acme": false
			}
		}`), 0o644)

		ep := LoadEnabledPlugins(f1, f2)
		if ep == nil {
			t.Fatal("expected non-nil")
		}
		// github: false in f1, true in f2 → true (OR)
		if !ep.IsEnabled("github") {
			t.Error("github should be enabled (OR merge)")
		}
		if !ep.IsEnabled("linter") {
			t.Error("linter should be enabled")
		}
		if ep.IsEnabled("formatter") {
			t.Error("formatter should be disabled")
		}
	})

	t.Run("invalid JSON ignored", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		good := filepath.Join(dir, "good.json")
		os.WriteFile(good, []byte(`{"enabledPlugins": {"github@x": true}}`), 0o644)
		bad := filepath.Join(dir, "bad.json")
		os.WriteFile(bad, []byte(`{invalid`), 0o644)

		ep := LoadEnabledPlugins(good, bad)
		if ep == nil {
			t.Fatal("expected non-nil")
		}
		if !ep.IsEnabled("github") {
			t.Error("github should be enabled from good file")
		}
	})
}

func TestEnabledPluginsIsEnabled(t *testing.T) {
	t.Parallel()

	t.Run("nil receiver returns true", func(t *testing.T) {
		t.Parallel()
		var ep *EnabledPlugins
		if !ep.IsEnabled("anything") {
			t.Error("nil receiver should return true")
		}
	})

	t.Run("enabled plugin returns true", func(t *testing.T) {
		t.Parallel()
		ep := &EnabledPlugins{names: map[string]bool{"github": true}}
		if !ep.IsEnabled("github") {
			t.Error("enabled plugin should return true")
		}
	})

	t.Run("disabled plugin returns false", func(t *testing.T) {
		t.Parallel()
		ep := &EnabledPlugins{names: map[string]bool{"linter": false}}
		if ep.IsEnabled("linter") {
			t.Error("disabled plugin should return false")
		}
	})

	t.Run("absent plugin returns true (conservative)", func(t *testing.T) {
		t.Parallel()
		ep := &EnabledPlugins{names: map[string]bool{"github": true}}
		if !ep.IsEnabled("unknown") {
			t.Error("absent plugin should return true (conservative)")
		}
	})
}

func TestEnabledPluginsMultipleMarketplaces(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "settings.json")
	os.WriteFile(file, []byte(`{
		"enabledPlugins": {
			"github@official": false,
			"github@community": true,
			"linter@acme": false,
			"linter@other": false
		}
	}`), 0o644)

	ep := LoadEnabledPlugins(file)
	if ep == nil {
		t.Fatal("expected non-nil")
	}
	// github: one true → enabled
	if !ep.IsEnabled("github") {
		t.Error("github should be enabled (one marketplace true)")
	}
	// linter: all false → disabled
	if ep.IsEnabled("linter") {
		t.Error("linter should be disabled (all marketplaces false)")
	}
}
