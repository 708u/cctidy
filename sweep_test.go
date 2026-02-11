package cctidy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractToolEntry(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		entry         string
		wantTool      string
		wantSpecifier string
	}{
		{
			name:          "Bash tool",
			entry:         "Bash(git -C /repo status)",
			wantTool:      "Bash",
			wantSpecifier: "git -C /repo status",
		},
		{
			name:          "Write tool",
			entry:         "Write(/some/path)",
			wantTool:      "Write",
			wantSpecifier: "/some/path",
		},
		{
			name:          "Read tool",
			entry:         "Read(/some/path)",
			wantTool:      "Read",
			wantSpecifier: "/some/path",
		},
		{
			name:          "mcp tool with underscores",
			entry:         "mcp__github__search_code(query)",
			wantTool:      "mcp__github__search_code",
			wantSpecifier: "query",
		},
		{
			name:     "bare tool name without parens",
			entry:    "Bash",
			wantTool: "",
		},
		{
			name:     "empty string",
			entry:    "",
			wantTool: "",
		},
		{
			name:     "starts with number",
			entry:    "1Tool(arg)",
			wantTool: "",
		},
		{
			name:          "WebFetch tool",
			entry:         "WebFetch(domain:github.com)",
			wantTool:      "WebFetch",
			wantSpecifier: "domain:github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotTool, gotSpec := extractToolEntry(tt.entry)
			if gotTool != tt.wantTool {
				t.Errorf("extractToolEntry(%q) tool = %q, want %q", tt.entry, gotTool, tt.wantTool)
			}
			if gotSpec != tt.wantSpecifier {
				t.Errorf("extractToolEntry(%q) specifier = %q, want %q", tt.entry, gotSpec, tt.wantSpecifier)
			}
		})
	}
}

func TestContainsGlob(t *testing.T) {
	t.Parallel()
	tests := []struct {
		s    string
		want bool
	}{
		{"**/*.ts", true},
		{"/path/to/file", false},
		{"src/[a-z]/*.go", true},
		{"file?.txt", true},
		{"normal/path", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			t.Parallel()
			if got := containsGlob(tt.s); got != tt.want {
				t.Errorf("containsGlob(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestReadEditToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sweeper   ReadEditToolSweeper
		specifier string
		wantSweep bool
	}{
		{
			name:      "absolute path with // prefix is resolved",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}},
			specifier: "//dead/path",
			wantSweep: true,
		},
		{
			name:      "existing absolute path is kept",
			sweeper:   ReadEditToolSweeper{checker: checkerFor("/alive/path")},
			specifier: "//alive/path",
			wantSweep: false,
		},
		{
			name:      "home-relative path with homeDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}, homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: true,
		},
		{
			name:      "existing home-relative path is kept",
			sweeper:   ReadEditToolSweeper{checker: checkerFor("/home/user/config.json"), homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "home-relative path without homeDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}, baseDir: "/project"},
			specifier: "./src/main.go",
			wantSweep: true,
		},
		{
			name:      "relative path without baseDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}},
			specifier: "./src/main.go",
			wantSweep: false,
		},
		{
			name:      "glob pattern is skipped",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}},
			specifier: "**/*.ts",
			wantSweep: false,
		},
		{
			name:      "parent-relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: alwaysFalse{}, baseDir: "/project"},
			specifier: "../other/file.go",
			wantSweep: true,
		},
		{
			name:      "slash-prefixed path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: checkerFor("/project/src/file.go"), baseDir: "/project"},
			specifier: "/src/file.go",
			wantSweep: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.sweeper.ShouldSweep(t.Context(), tt.specifier)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
			}
		})
	}
}

func TestSweepPermissions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		entries        []any
		checker        PathChecker
		homeDir        string
		opts           []SweepOption
		wantAllowLen   int
		wantSweptAllow int
	}{
		{
			name:           "dead absolute path entry is removed",
			entries:        []any{"Read(//dead/path)"},
			checker:        alwaysFalse{},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "existing absolute path entry is kept",
			entries:        []any{"Read(//alive/path)"},
			checker:        checkerFor("/alive/path"),
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "home-relative path with homeDir is swept when dead",
			entries:        []any{"Read(~/dead/config)"},
			checker:        alwaysFalse{},
			homeDir:        "/home/user",
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "home-relative path with homeDir is kept when exists",
			entries:        []any{"Read(~/config)"},
			checker:        checkerFor("/home/user/config"),
			homeDir:        "/home/user",
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path without baseDir is kept",
			entries:        []any{"Edit(/src/file.go)"},
			checker:        alwaysFalse{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path with baseDir is swept when dead",
			entries:        []any{"Edit(./src/file.go)"},
			checker:        alwaysFalse{},
			opts:           []SweepOption{WithBaseDir("/project")},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "glob pattern entry is kept",
			entries:        []any{"Read(**/*.ts)"},
			checker:        alwaysFalse{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name: "unregistered tool entries are kept",
			entries: []any{
				"Bash(git -C /dead/path status)",
				"WebFetch(domain:example.com)",
			},
			checker:        alwaysFalse{},
			wantAllowLen:   2,
			wantSweptAllow: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			obj := map[string]any{
				"permissions": map[string]any{
					"allow": tt.entries,
				},
			}
			result := NewPermissionSweeper(tt.checker, tt.homeDir, tt.opts...).Sweep(t.Context(), obj)
			allow := obj["permissions"].(map[string]any)["allow"].([]any)
			if len(allow) != tt.wantAllowLen {
				t.Errorf("allow len = %d, want %d, got %v", len(allow), tt.wantAllowLen, allow)
			}
			if result.SweptAllow != tt.wantSweptAllow {
				t.Errorf("SweptAllow = %d, want %d", result.SweptAllow, tt.wantSweptAllow)
			}
		})
	}

	t.Run("relative path with baseDir is kept when exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.go"), []byte(""), 0o644)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Edit(./file.go)"},
			},
		}
		result := NewPermissionSweeper(checkerFor(filepath.Join(dir, "file.go")), "", WithBaseDir(dir)).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("missing permissions key is no-op", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{"key": "value"}
		result := NewPermissionSweeper(alwaysTrue{}, "").Sweep(t.Context(), obj)
		if result.SweptAllow != 0 || result.SweptAsk != 0 {
			t.Errorf("expected zero counts, got allow=%d ask=%d",
				result.SweptAllow, result.SweptAsk)
		}
		if len(result.Warns) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warns)
		}
	})

	t.Run("deny entries are never swept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(//dead/allow)"},
				"deny":  []any{"Read(//dead/deny)"},
				"ask":   []any{"Edit(//dead/ask)"},
			},
		}
		result := NewPermissionSweeper(alwaysFalse{}, "").Sweep(t.Context(), obj)
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
		if result.SweptAsk != 1 {
			t.Errorf("SweptAsk = %d, want 1", result.SweptAsk)
		}
		deny := obj["permissions"].(map[string]any)["deny"].([]any)
		if len(deny) != 1 {
			t.Errorf("deny should be kept unchanged, got %v", deny)
		}
	})
}
