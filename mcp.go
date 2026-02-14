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

// LoadMCPServers collects known MCP server names from .mcp.json
// and ~/.claude.json. Missing files are silently ignored.
// JSON parse errors are returned.
func LoadMCPServers(mcpJSONPath, claudeJSONPath string) (MCPServerSet, error) {
	servers := make(MCPServerSet)

	if err := loadMCPServersFromMCPJSON(mcpJSONPath, servers); err != nil {
		return nil, err
	}
	if err := loadMCPServersFromClaudeJSON(claudeJSONPath, servers); err != nil {
		return nil, err
	}

	return servers, nil
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

	raw, ok := obj["mcpServers"]
	if !ok {
		return nil
	}
	var mcpServers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &mcpServers); err != nil {
		return nil
	}
	for name := range mcpServers {
		servers[name] = true
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
		var topServers map[string]json.RawMessage
		if err := json.Unmarshal(raw, &topServers); err == nil {
			for name := range topServers {
				servers[name] = true
			}
		}
	}

	// projects → each project → mcpServers
	if raw, ok := obj["projects"]; ok {
		var projects map[string]json.RawMessage
		if err := json.Unmarshal(raw, &projects); err == nil {
			for _, projRaw := range projects {
				var projObj map[string]json.RawMessage
				if err := json.Unmarshal(projRaw, &projObj); err != nil {
					continue
				}
				if mcpRaw, ok := projObj["mcpServers"]; ok {
					var projServers map[string]json.RawMessage
					if err := json.Unmarshal(mcpRaw, &projServers); err == nil {
						for name := range projServers {
							servers[name] = true
						}
					}
				}
			}
		}
	}

	return nil
}

// MCPExcluder decides whether an MCP server should be excluded from sweeping.
type MCPExcluder struct {
	servers map[string]bool
}

// NewMCPExcluder builds an MCPExcluder from exclude server names.
func NewMCPExcluder(excludeServers []string) *MCPExcluder {
	m := make(map[string]bool, len(excludeServers))
	for _, s := range excludeServers {
		m[s] = true
	}
	return &MCPExcluder{servers: m}
}

// IsExcluded reports whether the server name is excluded.
func (e *MCPExcluder) IsExcluded(serverName string) bool {
	return e.servers[serverName]
}

// MCPToolSweeper sweeps MCP tool permission entries whose server
// is no longer present in the known server set.
type MCPToolSweeper struct {
	servers  MCPServerSet
	excluder *MCPExcluder
}

// NewMCPToolSweeper creates an MCPToolSweeper.
func NewMCPToolSweeper(servers MCPServerSet, excluder *MCPExcluder) *MCPToolSweeper {
	return &MCPToolSweeper{servers: servers, excluder: excluder}
}

// ShouldSweep implements ToolSweeper. The specifier is the full
// MCP tool name (e.g. "mcp__slack__post_message").
// Returns Sweep=true when the server is not in the known set
// and not excluded.
func (m *MCPToolSweeper) ShouldSweep(_ context.Context, toolName string) ToolSweepResult {
	serverName, ok := extractMCPServerName(toolName)
	if !ok {
		return ToolSweepResult{}
	}
	if m.excluder.IsExcluded(serverName) {
		return ToolSweepResult{}
	}
	if m.servers[serverName] {
		return ToolSweepResult{}
	}
	return ToolSweepResult{Sweep: true}
}
