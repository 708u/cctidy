package cctidy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/708u/cctidy/internal/testutil"
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

func TestExtractAbsolutePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single absolute path",
			input: "git -C /home/user/repo status",
			want:  []string{"/home/user/repo"},
		},
		{
			name:  "glob stops extraction",
			input: "npm run *",
			want:  nil,
		},
		{
			name:  "shell metachar stops at &&",
			input: "cd /path && make",
			want:  []string{"/path"},
		},
		{
			name:  "semicolon stops extraction",
			input: "cd /path;make",
			want:  []string{"/path"},
		},
		{
			name:  "equals-prefixed path",
			input: "--config=/etc/app.conf",
			want:  []string{"/etc/app.conf"},
		},
		{
			name:  "multiple paths",
			input: "cp /src/a /dst/b",
			want:  []string{"/src/a", "/dst/b"},
		},
		{
			name:  "no absolute paths",
			input: "echo hello",
			want:  nil,
		},
		{
			name:  "trailing slash trimmed",
			input: "ls /some/dir/",
			want:  []string{"/some/dir"},
		},
		{
			name:  "trailing dot trimmed",
			input: "ls /some/path.",
			want:  []string{"/some/path"},
		},
		{
			name:  "root only path filtered",
			input: "ls /",
			want:  nil,
		},
		{
			name:  "path with underscores and dots",
			input: "cat /home/user/.config/app_v2/settings.json",
			want:  []string{"/home/user/.config/app_v2/settings.json"},
		},
		{
			name:  "dot-slash relative path is not extracted",
			input: "./bin/run",
			want:  nil,
		},
		{
			name:  "dot-dot-slash relative path is not extracted",
			input: "../src/main.go",
			want:  nil,
		},
		{
			name:  "tilde home path is not extracted",
			input: "cat ~/config.json",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractAbsolutePaths(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("extractAbsolutePaths(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("extractAbsolutePaths(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestBashToolSweeperShouldSweep(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		checker   PathChecker
		specifier string
		wantSweep bool
	}{
		{
			name:      "all paths dead",
			checker:   testutil.NoPathsExist{},
			specifier: "git -C /dead/repo status",
			wantSweep: true,
		},
		{
			name:      "one path alive",
			checker:   testutil.CheckerFor("/alive/src"),
			specifier: "cp /alive/src /dead/dst",
			wantSweep: false,
		},
		{
			name:      "no absolute paths keeps entry",
			checker:   testutil.NoPathsExist{},
			specifier: "npm run *",
			wantSweep: false,
		},
		{
			name:      "all paths alive",
			checker:   testutil.AllPathsExist{},
			specifier: "cp /src/a /dst/b",
			wantSweep: false,
		},
		{
			name:      "multiple dead paths",
			checker:   testutil.NoPathsExist{},
			specifier: "cp /dead/a /dead/b",
			wantSweep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sweeper := BashToolSweeper{checker: tt.checker}
			result := sweeper.ShouldSweep(t.Context(), tt.specifier)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%q) = %v, want %v", tt.specifier, result.Sweep, tt.wantSweep)
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
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "//dead/path",
			wantSweep: true,
		},
		{
			name:      "existing absolute path is kept",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/alive/path")},
			specifier: "//alive/path",
			wantSweep: false,
		},
		{
			name:      "home-relative path with homeDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: true,
		},
		{
			name:      "existing home-relative path is kept",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/home/user/config.json"), homeDir: "/home/user"},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "home-relative path without homeDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "~/config.json",
			wantSweep: false,
		},
		{
			name:      "relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project"},
			specifier: "./src/main.go",
			wantSweep: true,
		},
		{
			name:      "relative path without baseDir is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "./src/main.go",
			wantSweep: false,
		},
		{
			name:      "glob pattern is skipped",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}},
			specifier: "**/*.ts",
			wantSweep: false,
		},
		{
			name:      "parent-relative path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.NoPathsExist{}, baseDir: "/project"},
			specifier: "../other/file.go",
			wantSweep: true,
		},
		{
			name:      "slash-prefixed path with baseDir is resolved",
			sweeper:   ReadEditToolSweeper{checker: testutil.CheckerFor("/project/src/file.go"), baseDir: "/project"},
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
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "existing absolute path entry is kept",
			entries:        []any{"Read(//alive/path)"},
			checker:        testutil.CheckerFor("/alive/path"),
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "home-relative path with homeDir is swept when dead",
			entries:        []any{"Read(~/dead/config)"},
			checker:        testutil.NoPathsExist{},
			homeDir:        "/home/user",
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "home-relative path with homeDir is kept when exists",
			entries:        []any{"Read(~/config)"},
			checker:        testutil.CheckerFor("/home/user/config"),
			homeDir:        "/home/user",
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path without baseDir is kept",
			entries:        []any{"Edit(/src/file.go)"},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name:           "relative path with baseDir is swept when dead",
			entries:        []any{"Edit(./src/file.go)"},
			checker:        testutil.NoPathsExist{},
			opts:           []SweepOption{WithBaseDir("/project")},
			wantAllowLen:   0,
			wantSweptAllow: 1,
		},
		{
			name:           "glob pattern entry is kept",
			entries:        []any{"Read(**/*.ts)"},
			checker:        testutil.NoPathsExist{},
			wantAllowLen:   1,
			wantSweptAllow: 0,
		},
		{
			name: "unregistered tool entries are kept",
			entries: []any{
				"Bash(git -C /dead/path status)",
				"WebFetch(domain:example.com)",
			},
			checker:        testutil.NoPathsExist{},
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
		result := NewPermissionSweeper(testutil.CheckerFor(filepath.Join(dir, "file.go")), "", WithBaseDir(dir)).Sweep(t.Context(), obj)
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
		result := NewPermissionSweeper(testutil.AllPathsExist{}, "").Sweep(t.Context(), obj)
		if result.SweptAllow != 0 || result.SweptAsk != 0 {
			t.Errorf("expected zero counts, got allow=%d ask=%d",
				result.SweptAllow, result.SweptAsk)
		}
		if len(result.Warns) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warns)
		}
	})

	t.Run("bash entries swept when enabled and all paths dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(git -C /dead/repo status)",
					"Bash(npm run *)",
					"Read",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "", WithBashSweep()).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 2 {
			t.Errorf("allow len = %d, want 2, got %v", len(allow), allow)
		}
		if result.SweptAllow != 1 {
			t.Errorf("SweptAllow = %d, want 1", result.SweptAllow)
		}
	})

	t.Run("bash entries kept when one path alive", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(cp /alive/src /dead/dst)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.CheckerFor("/alive/src"), "", WithBashSweep()).Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
		}
	})

	t.Run("bash entries kept when sweep-bash not enabled", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(git -C /dead/repo status)",
				},
			},
		}
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "").Sweep(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow len = %d, want 1, got %v", len(allow), allow)
		}
		if result.SweptAllow != 0 {
			t.Errorf("SweptAllow = %d, want 0", result.SweptAllow)
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
		result := NewPermissionSweeper(testutil.NoPathsExist{}, "").Sweep(t.Context(), obj)
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
