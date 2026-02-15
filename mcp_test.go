package cctidy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMCPServerName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		toolName   string
		wantServer string
		wantOK     bool
	}{
		{
			name:       "standard MCP tool",
			toolName:   "mcp__slack__post_message",
			wantServer: "slack",
			wantOK:     true,
		},
		{
			name:       "bare MCP server name",
			toolName:   "mcp__slack",
			wantServer: "slack",
			wantOK:     true,
		},
		{
			name:       "MCP tool with multiple segments",
			toolName:   "mcp__github__search_code",
			wantServer: "github",
			wantOK:     true,
		},
		{
			name:       "plugin entry excluded",
			toolName:   "mcp__plugin_github_github__search_code",
			wantServer: "",
			wantOK:     false,
		},
		{
			name:       "non-MCP tool",
			toolName:   "Read",
			wantServer: "",
			wantOK:     false,
		},
		{
			name:       "Bash tool",
			toolName:   "Bash",
			wantServer: "",
			wantOK:     false,
		},
		{
			name:       "empty string",
			toolName:   "",
			wantServer: "",
			wantOK:     false,
		},
		{
			name:       "mcp__ only",
			toolName:   "mcp__",
			wantServer: "",
			wantOK:     false,
		},
		{
			name:       "MCP tool with hyphen in server name",
			toolName:   "mcp__my-server__tool",
			wantServer: "my-server",
			wantOK:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotServer, gotOK := extractMCPServerName(tt.toolName)
			if gotServer != tt.wantServer {
				t.Errorf("extractMCPServerName(%q) server = %q, want %q", tt.toolName, gotServer, tt.wantServer)
			}
			if gotOK != tt.wantOK {
				t.Errorf("extractMCPServerName(%q) ok = %v, want %v", tt.toolName, gotOK, tt.wantOK)
			}
		})
	}
}

func TestLoadMCPServers(t *testing.T) {
	t.Parallel()

	t.Run("both files present", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{
			"mcpServers": {
				"slack": {"type": "stdio"},
				"github": {"type": "stdio"}
			}
		}`), 0o644)

		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{
			"mcpServers": {
				"jira": {"type": "stdio"}
			},
			"projects": {
				"/project-a": {
					"mcpServers": {
						"sentry": {"type": "http"}
					}
				}
			}
		}`), 0o644)

		servers, err := LoadMCPServers(mcpJSON, claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, name := range []string{"slack", "github", "jira", "sentry"} {
			if !servers[name] {
				t.Errorf("expected server %q to be present", name)
			}
		}
	})

	t.Run("missing files returns empty set", func(t *testing.T) {
		t.Parallel()
		servers, err := LoadMCPServers("/nonexistent/.mcp.json", "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(servers) != 0 {
			t.Errorf("expected empty set, got %v", servers)
		}
	})

	t.Run("mcp.json only", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{
			"mcpServers": {
				"slack": {"type": "stdio"}
			}
		}`), 0o644)

		servers, err := LoadMCPServers(mcpJSON, "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !servers["slack"] {
			t.Error("expected slack server")
		}
		if len(servers) != 1 {
			t.Errorf("expected 1 server, got %d", len(servers))
		}
	})

	t.Run("claude.json only with projects", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{
			"projects": {
				"/proj": {
					"mcpServers": {
						"slack": {},
						"github": {}
					}
				}
			}
		}`), 0o644)

		servers, err := LoadMCPServers("/nonexistent/.mcp.json", claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !servers["slack"] || !servers["github"] {
			t.Errorf("expected slack and github, got %v", servers)
		}
	})

	t.Run("invalid mcp.json returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{invalid`), 0o644)

		_, err := LoadMCPServers(mcpJSON, "/nonexistent/.claude.json")
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("invalid claude.json returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{invalid`), 0o644)

		_, err := LoadMCPServers("/nonexistent/.mcp.json", claudeJSON)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("no mcpServers key returns empty set", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{"other": "value"}`), 0o644)

		servers, err := LoadMCPServers(mcpJSON, "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(servers) != 0 {
			t.Errorf("expected empty set, got %v", servers)
		}
	})

	t.Run("union of all three sources", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{
			"mcpServers": {"from-mcp": {}}
		}`), 0o644)

		claudeJSON := filepath.Join(dir, ".claude.json")
		os.WriteFile(claudeJSON, []byte(`{
			"mcpServers": {"from-top": {}},
			"projects": {
				"/p": {"mcpServers": {"from-project": {}}}
			}
		}`), 0o644)

		servers, err := LoadMCPServers(mcpJSON, claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, name := range []string{"from-mcp", "from-top", "from-project"} {
			if !servers[name] {
				t.Errorf("expected server %q", name)
			}
		}
	})
}

func TestMCPToolSweeper(t *testing.T) {
	t.Parallel()

	servers := MCPServerSet{"slack": true, "github": true}

	tests := []struct {
		name      string
		sweeper   *MCPToolSweeper
		entry     MCPEntry
		wantSweep bool
	}{
		{
			name:      "server exists - keep",
			sweeper:   NewMCPToolSweeper(servers),
			entry:     MCPEntry{ServerName: "slack", RawEntry: "mcp__slack__post_message"},
			wantSweep: false,
		},
		{
			name:      "server missing - sweep",
			sweeper:   NewMCPToolSweeper(servers),
			entry:     MCPEntry{ServerName: "jira", RawEntry: "mcp__jira__create_issue"},
			wantSweep: true,
		},
		{
			name:      "bare server name exists - keep",
			sweeper:   NewMCPToolSweeper(servers),
			entry:     MCPEntry{ServerName: "github", RawEntry: "mcp__github"},
			wantSweep: false,
		},
		{
			name:      "bare server name missing - sweep",
			sweeper:   NewMCPToolSweeper(servers),
			entry:     MCPEntry{ServerName: "sentry", RawEntry: "mcp__sentry"},
			wantSweep: true,
		},
		{
			name:      "empty server set sweeps unknown",
			sweeper:   NewMCPToolSweeper(MCPServerSet{}),
			entry:     MCPEntry{ServerName: "slack", RawEntry: "mcp__slack__post_message"},
			wantSweep: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.sweeper.ShouldSweep(t.Context(), tt.entry)
			if result.Sweep != tt.wantSweep {
				t.Errorf("ShouldSweep(%v) = %v, want %v", tt.entry, result.Sweep, tt.wantSweep)
			}
		})
	}
}
