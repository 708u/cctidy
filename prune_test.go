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

func TestReadEditToolPrunerShouldPrune(t *testing.T) {
	t.Parallel()

	t.Run("absolute path with // prefix is resolved", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{checker: alwaysFalse{}}
		result := p.ShouldPrune(t.Context(), "//dead/path")
		if !result.Prune {
			t.Error("should prune non-existent absolute path")
		}
	})

	t.Run("existing absolute path is kept", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{checker: checkerFor("/alive/path")}
		result := p.ShouldPrune(t.Context(), "//alive/path")
		if result.Prune {
			t.Error("should not prune existing absolute path")
		}
	})

	t.Run("home-relative path with homeDir is resolved", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{
			checker: alwaysFalse{},
			homeDir: "/home/user",
		}
		result := p.ShouldPrune(t.Context(), "~/config.json")
		if !result.Prune {
			t.Error("should prune non-existent home-relative path")
		}
	})

	t.Run("existing home-relative path is kept", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{
			checker: checkerFor("/home/user/config.json"),
			homeDir: "/home/user",
		}
		result := p.ShouldPrune(t.Context(), "~/config.json")
		if result.Prune {
			t.Error("should not prune existing home-relative path")
		}
	})

	t.Run("home-relative path without homeDir is skipped", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{checker: alwaysFalse{}}
		result := p.ShouldPrune(t.Context(), "~/config.json")
		if result.Prune {
			t.Error("should skip home-relative path without homeDir")
		}
	})

	t.Run("relative path with baseDir is resolved", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{
			checker: alwaysFalse{},
			baseDir: "/project",
		}
		result := p.ShouldPrune(t.Context(), "./src/main.go")
		if !result.Prune {
			t.Error("should prune non-existent relative path")
		}
	})

	t.Run("relative path without baseDir is skipped", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{checker: alwaysFalse{}}
		result := p.ShouldPrune(t.Context(), "./src/main.go")
		if result.Prune {
			t.Error("should skip relative path without baseDir")
		}
	})

	t.Run("glob pattern is skipped", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{checker: alwaysFalse{}}
		result := p.ShouldPrune(t.Context(), "**/*.ts")
		if result.Prune {
			t.Error("should skip glob pattern")
		}
	})

	t.Run("slash-prefixed path with baseDir is resolved", func(t *testing.T) {
		t.Parallel()
		p := &ReadEditToolPruner{
			checker: checkerFor("/project/src/file.go"),
			baseDir: "/project",
		}
		result := p.ShouldPrune(t.Context(), "/src/file.go")
		if result.Prune {
			t.Error("should not prune existing path resolved with baseDir")
		}
	})
}

func TestPrunePermissions(t *testing.T) {
	t.Parallel()

	t.Run("dead absolute path entry is removed", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(//dead/path)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow should be empty, got %v", allow)
		}
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
	})

	t.Run("existing absolute path entry is kept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(//alive/path)"},
			},
		}
		result := NewPermissionPruner(checkerFor("/alive/path")).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("home-relative path with homeDir is pruned when dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(~/dead/config)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}, WithHomeDir("/home/user")).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow should be empty, got %v", allow)
		}
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
	})

	t.Run("home-relative path with homeDir is kept when exists", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(~/config)"},
			},
		}
		result := NewPermissionPruner(checkerFor("/home/user/config"), WithHomeDir("/home/user")).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("relative path without baseDir is kept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Edit(/src/file.go)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("relative path with baseDir is pruned when dead", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Edit(./src/file.go)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}, WithBaseDir("/project")).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow should be empty, got %v", allow)
		}
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
	})

	t.Run("relative path with baseDir is kept when exists", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.go"), []byte(""), 0o644)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Edit(./file.go)"},
			},
		}
		result := NewPermissionPruner(checkerFor(filepath.Join(dir, "file.go")), WithBaseDir(dir)).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("glob pattern entry is kept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(**/*.ts)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("unregistered tool entries are kept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{
					"Bash(git -C /dead/path status)",
					"WebFetch(domain:example.com)",
				},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}).Prune(t.Context(), obj)
		allow := obj["permissions"].(map[string]any)["allow"].([]any)
		if len(allow) != 2 {
			t.Errorf("allow should have 2 entries, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("missing permissions key is no-op", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{"key": "value"}
		result := NewPermissionPruner(alwaysTrue{}).Prune(t.Context(), obj)
		if result.PrunedAllow != 0 || result.PrunedAsk != 0 {
			t.Errorf("expected zero counts, got allow=%d ask=%d",
				result.PrunedAllow, result.PrunedAsk)
		}
		if len(result.Warns) != 0 {
			t.Errorf("expected no warnings, got %v", result.Warns)
		}
	})

	t.Run("deny entries are never pruned", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Read(//dead/allow)"},
				"deny":  []any{"Read(//dead/deny)"},
				"ask":   []any{"Edit(//dead/ask)"},
			},
		}
		result := NewPermissionPruner(alwaysFalse{}).Prune(t.Context(), obj)
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
		if result.PrunedAsk != 1 {
			t.Errorf("PrunedAsk = %d, want 1", result.PrunedAsk)
		}
		deny := obj["permissions"].(map[string]any)["deny"].([]any)
		if len(deny) != 1 {
			t.Errorf("deny should be kept unchanged, got %v", deny)
		}
	})
}
