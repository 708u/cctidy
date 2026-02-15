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

		sets, err := LoadMCPServers(mcpJSON, claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		user := sets.ForUserScope()
		project := sets.ForProjectScope()

		// User scope: only claude.json servers
		for _, name := range []string{"jira", "sentry"} {
			if !user[name] {
				t.Errorf("user scope: expected server %q", name)
			}
		}
		for _, name := range []string{"slack", "github"} {
			if user[name] {
				t.Errorf("user scope: should not contain %q", name)
			}
		}

		// Project scope: all servers
		for _, name := range []string{"slack", "github", "jira", "sentry"} {
			if !project[name] {
				t.Errorf("project scope: expected server %q", name)
			}
		}
	})

	t.Run("missing files returns empty sets", func(t *testing.T) {
		t.Parallel()
		sets, err := LoadMCPServers("/nonexistent/.mcp.json", "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sets.ForUserScope()) != 0 {
			t.Errorf("expected empty user set, got %v", sets.ForUserScope())
		}
		if len(sets.ForProjectScope()) != 0 {
			t.Errorf("expected empty project set, got %v", sets.ForProjectScope())
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

		sets, err := LoadMCPServers(mcpJSON, "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// User scope should be empty (no claude.json servers)
		if len(sets.ForUserScope()) != 0 {
			t.Errorf("user scope should be empty, got %v", sets.ForUserScope())
		}
		// Project scope should contain slack
		project := sets.ForProjectScope()
		if !project["slack"] {
			t.Error("project scope: expected slack server")
		}
		if len(project) != 1 {
			t.Errorf("project scope: expected 1 server, got %d", len(project))
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

		sets, err := LoadMCPServers("/nonexistent/.mcp.json", claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		user := sets.ForUserScope()
		if !user["slack"] || !user["github"] {
			t.Errorf("user scope: expected slack and github, got %v", user)
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

	t.Run("no mcpServers key returns empty sets", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		mcpJSON := filepath.Join(dir, ".mcp.json")
		os.WriteFile(mcpJSON, []byte(`{"other": "value"}`), 0o644)

		sets, err := LoadMCPServers(mcpJSON, "/nonexistent/.claude.json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sets.ForUserScope()) != 0 {
			t.Errorf("expected empty user set, got %v", sets.ForUserScope())
		}
		if len(sets.ForProjectScope()) != 0 {
			t.Errorf("expected empty project set, got %v", sets.ForProjectScope())
		}
	})

	t.Run("scope separation of sources", func(t *testing.T) {
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

		sets, err := LoadMCPServers(mcpJSON, claudeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		user := sets.ForUserScope()
		project := sets.ForProjectScope()

		// User scope: only claude.json sources
		if user["from-mcp"] {
			t.Error("user scope should not contain from-mcp")
		}
		for _, name := range []string{"from-top", "from-project"} {
			if !user[name] {
				t.Errorf("user scope: expected %q", name)
			}
		}

		// Project scope: all sources
		for _, name := range []string{"from-mcp", "from-top", "from-project"} {
			if !project[name] {
				t.Errorf("project scope: expected %q", name)
			}
		}
	})
}

func TestMCPServerSetsScopes(t *testing.T) {
	t.Parallel()

	t.Run("nil pointer returns empty sets", func(t *testing.T) {
		t.Parallel()
		var sets *MCPServerSets
		user := sets.ForUserScope()
		project := sets.ForProjectScope()
		if len(user) != 0 {
			t.Errorf("nil user scope should be empty, got %v", user)
		}
		if len(project) != 0 {
			t.Errorf("nil project scope should be empty, got %v", project)
		}
	})

	t.Run("user scope excludes mcp.json", func(t *testing.T) {
		t.Parallel()
		sets := &MCPServerSets{
			mcpJSON:    MCPServerSet{"slack": true},
			claudeJSON: MCPServerSet{"github": true},
		}
		user := sets.ForUserScope()
		if user["slack"] {
			t.Error("user scope should not contain mcp.json server")
		}
		if !user["github"] {
			t.Error("user scope should contain claude.json server")
		}
	})

	t.Run("project scope includes both", func(t *testing.T) {
		t.Parallel()
		sets := &MCPServerSets{
			mcpJSON:    MCPServerSet{"slack": true},
			claudeJSON: MCPServerSet{"github": true},
		}
		project := sets.ForProjectScope()
		if !project["slack"] {
			t.Error("project scope should contain mcp.json server")
		}
		if !project["github"] {
			t.Error("project scope should contain claude.json server")
		}
	})

	t.Run("returned sets are independent copies", func(t *testing.T) {
		t.Parallel()
		sets := &MCPServerSets{
			claudeJSON: MCPServerSet{"github": true},
		}
		user1 := sets.ForUserScope()
		user2 := sets.ForUserScope()
		user1["modified"] = true
		if user2["modified"] {
			t.Error("modifying one returned set should not affect another")
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
