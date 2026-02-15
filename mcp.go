package cctidy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// MCPServerSet is a set of known MCP server names.
type MCPServerSet map[string]bool

// extractMCPServerName extracts the server name from an MCP tool name.
// Returns ("", false) for non-MCP tool names or plugin entries (mcp__plugin_*).
//
// Examples:
//   - "mcp__slack__tool"   → ("slack", true)
//   - "mcp__slack"         → ("slack", true)
//   - "mcp__plugin_github_github__tool" → ("", false)
//   - "Read"               → ("", false)
func extractMCPServerName(toolName string) (string, bool) {
	if !strings.HasPrefix(toolName, "mcp__") {
		return "", false
	}
	rest := toolName[len("mcp__"):]
	if strings.HasPrefix(rest, "plugin_") {
		return "", false
	}
	// Server name is the segment before the next "__", or the entire rest.
	if idx := strings.Index(rest, "__"); idx > 0 {
		return rest[:idx], true
	}
	if rest != "" {
		return rest, true
	}
	return "", false
}

// MCPServerSets holds MCP server names separated by source.
// Use ForUserScope or ForProjectScope to get the appropriate
// set for a given settings file scope.
type MCPServerSets struct {
	mcpJSON    MCPServerSet // from .mcp.json
	claudeJSON MCPServerSet // from ~/.claude.json
}

// ForUserScope returns servers available in user scope
// (~/.claude/settings.json, ~/.claude/settings.local.json).
// This includes only ~/.claude.json servers, not .mcp.json.
func (s *MCPServerSets) ForUserScope() MCPServerSet {
	if s == nil || len(s.claudeJSON) == 0 {
		return MCPServerSet{}
	}
	result := make(MCPServerSet, len(s.claudeJSON))
	for k := range s.claudeJSON {
		result[k] = true
	}
	return result
}

// ForProjectScope returns servers available in project scope
// (.claude/settings.json, .claude/settings.local.json).
// This includes both .mcp.json and ~/.claude.json servers.
func (s *MCPServerSets) ForProjectScope() MCPServerSet {
	if s == nil {
		return MCPServerSet{}
	}
	result := make(MCPServerSet, len(s.mcpJSON)+len(s.claudeJSON))
	for k := range s.claudeJSON {
		result[k] = true
	}
	for k := range s.mcpJSON {
		result[k] = true
	}
	return result
}

// LoadMCPServers collects known MCP server names from .mcp.json
// and ~/.claude.json. Missing files are silently ignored.
// JSON parse errors are returned.
func LoadMCPServers(mcpJSONPath, claudeJSONPath string) (*MCPServerSets, error) {
	mcpServers := make(MCPServerSet)
	claudeServers := make(MCPServerSet)

	if err := loadMCPServersFromMCPJSON(mcpJSONPath, mcpServers); err != nil {
		return nil, err
	}
	if err := loadMCPServersFromClaudeJSON(claudeJSONPath, claudeServers); err != nil {
		return nil, err
	}

	return &MCPServerSets{mcpJSON: mcpServers, claudeJSON: claudeServers}, nil
}

// collectServerNames unmarshals raw as a JSON object and adds
// its keys to servers.
func collectServerNames(raw json.RawMessage, servers MCPServerSet) {
	var names map[string]json.RawMessage
	if err := json.Unmarshal(raw, &names); err != nil {
		return
	}
	for name := range names {
		servers[name] = true
	}
}

// loadMCPServersFromMCPJSON reads .mcp.json and extracts server names
// from the "mcpServers" key.
func loadMCPServersFromMCPJSON(path string, servers MCPServerSet) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	if raw, ok := obj["mcpServers"]; ok {
		collectServerNames(raw, servers)
	}
	return nil
}

// loadMCPServersFromClaudeJSON reads ~/.claude.json and extracts server
// names from the top-level "mcpServers" key and from
// "projects" → each project → "mcpServers".
func loadMCPServersFromClaudeJSON(path string, servers MCPServerSet) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	// Top-level mcpServers
	if raw, ok := obj["mcpServers"]; ok {
		collectServerNames(raw, servers)
	}

	// projects → each project → mcpServers
	raw, ok := obj["projects"]
	if !ok {
		return nil
	}
	var projects map[string]json.RawMessage
	if err := json.Unmarshal(raw, &projects); err != nil {
		return nil
	}
	for _, projRaw := range projects {
		var projObj map[string]json.RawMessage
		if err := json.Unmarshal(projRaw, &projObj); err != nil {
			continue
		}
		if mcpRaw, ok := projObj["mcpServers"]; ok {
			collectServerNames(mcpRaw, servers)
		}
	}

	return nil
}

// MCPToolSweeper sweeps MCP tool permission entries whose server
// is no longer present in the known server set.
type MCPToolSweeper struct {
	servers MCPServerSet
}

// NewMCPToolSweeper creates an MCPToolSweeper.
func NewMCPToolSweeper(servers MCPServerSet) *MCPToolSweeper {
	return &MCPToolSweeper{servers: servers}
}

// ShouldSweep evaluates an MCPEntry. Returns Sweep=true when the
// server is not in the known set.
func (m *MCPToolSweeper) ShouldSweep(_ context.Context, entry MCPEntry) ToolSweepResult {
	if m.servers[entry.ServerName] {
		return ToolSweepResult{}
	}
	return ToolSweepResult{Sweep: true}
}
