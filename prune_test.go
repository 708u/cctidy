package cctidy

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestExtractAbsolutePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		entry string
		want  []string
	}{
		{
			name:  "git command with repo path",
			entry: "Bash(git -C /Users/foo/repo status)",
			want:  []string{"/Users/foo/repo"},
		},
		{
			name:  "direct binary path with glob",
			entry: "Bash(/Users/foo/bin/mytool run:*)",
			want:  []string{"/Users/foo/bin/mytool"},
		},
		{
			name:  "export with path value",
			entry: "Bash(export VAR=/Users/foo/proj)",
			want:  []string{"/Users/foo/proj"},
		},
		{
			name:  "no paths in npm command",
			entry: "Bash(npm run *)",
			want:  nil,
		},
		{
			name:  "mcp tool name",
			entry: "mcp__github__search_code",
			want:  nil,
		},
		{
			name:  "domain-based web fetch",
			entry: "WebFetch(domain:github.com)",
			want:  nil,
		},
		{
			name:  "relative path not matched",
			entry: "Bash(./ccfmt:*)",
			want:  nil,
		},
		{
			name:  "relative path segment not matched",
			entry: "Bash(git show HEAD:api/pkg/file.go)",
			want:  nil,
		},
		{
			name:  "multiple absolute paths",
			entry: "Bash(cp /src/file /dst/file)",
			want:  []string{"/src/file", "/dst/file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractAbsolutePaths(tt.entry)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractAbsolutePaths(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestExtractRelativePaths(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		entry string
		want  []string
	}{
		{
			name:  "relative path present",
			entry: "Bash(./ccfmt:*)",
			want:  []string{"./ccfmt"},
		},
		{
			name:  "no relative path",
			entry: "Bash(npm run *)",
			want:  nil,
		},
		{
			name:  "absolute path only",
			entry: "Bash(/usr/bin/echo)",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ExtractRelativePaths(tt.entry)
			if !slices.Equal(got, tt.want) {
				t.Errorf("ExtractRelativePaths(%q) = %v, want %v", tt.entry, got, tt.want)
			}
		})
	}
}

func TestPrunePermissions(t *testing.T) {
	t.Parallel()

	t.Run("dead path entry is removed", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(git -C /Users/foo/repo status)"},
			},
		}
		result := prunePermissions(t.Context(), obj, alwaysFalse{}, "")
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		if len(allow) != 0 {
			t.Errorf("allow should be empty, got %v", allow)
		}
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
	})

	t.Run("existing path entry is kept", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(git -C /Users/foo/repo status)"},
			},
		}
		result := prunePermissions(t.Context(), obj, checkerFor("/Users/foo/repo"), "")
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("entry without paths is kept unchanged", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(npm run *)"},
			},
		}
		result := prunePermissions(t.Context(), obj, alwaysFalse{}, "")
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("relative path without baseDir is warned", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(./ccfmt:*)"},
			},
		}
		result := prunePermissions(t.Context(), obj, alwaysFalse{}, "")
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if len(result.RelativeWarns) != 1 || result.RelativeWarns[0] != "Bash(./ccfmt:*)" {
			t.Errorf("RelativeWarns = %v, want [Bash(./ccfmt:*)]", result.RelativeWarns)
		}
	})

	t.Run("relative path with baseDir is resolved and pruned", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(./ccfmt:*)"},
			},
		}
		result := prunePermissions(t.Context(), obj, alwaysFalse{}, "/project")
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
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
		os.WriteFile(filepath.Join(dir, "ccfmt"), []byte(""), 0o755)

		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(./ccfmt:*)"},
			},
		}
		result := prunePermissions(t.Context(), obj, checkerFor(filepath.Join(dir, "ccfmt")), dir)
		perms := obj["permissions"].(map[string]any)
		allow := perms["allow"].([]any)
		if len(allow) != 1 {
			t.Errorf("allow should have 1 entry, got %v", allow)
		}
		if result.PrunedAllow != 0 {
			t.Errorf("PrunedAllow = %d, want 0", result.PrunedAllow)
		}
	})

	t.Run("missing permissions key is no-op", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{"key": "value"}
		result := prunePermissions(t.Context(), obj, alwaysTrue{}, "")
		if result.PrunedAllow != 0 || result.PrunedDeny != 0 || result.PrunedAsk != 0 {
			t.Errorf("expected zero counts, got allow=%d deny=%d ask=%d",
				result.PrunedAllow, result.PrunedDeny, result.PrunedAsk)
		}
		if len(result.RelativeWarns) != 0 {
			t.Errorf("expected no warnings, got %v", result.RelativeWarns)
		}
	})

	t.Run("all three categories", func(t *testing.T) {
		t.Parallel()
		obj := map[string]any{
			"permissions": map[string]any{
				"allow": []any{"Bash(git -C /dead/allow status)"},
				"deny":  []any{"Bash(git -C /dead/deny status)"},
				"ask":   []any{"Bash(git -C /dead/ask status)"},
			},
		}
		result := prunePermissions(t.Context(), obj, alwaysFalse{}, "")
		if result.PrunedAllow != 1 {
			t.Errorf("PrunedAllow = %d, want 1", result.PrunedAllow)
		}
		if result.PrunedDeny != 1 {
			t.Errorf("PrunedDeny = %d, want 1", result.PrunedDeny)
		}
		if result.PrunedAsk != 1 {
			t.Errorf("PrunedAsk = %d, want 1", result.PrunedAsk)
		}
	})
}
