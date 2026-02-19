package cctidy

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/708u/cctidy/internal/set"
)

// ReduceResult holds statistics from a single reducer pass.
type ReduceResult struct {
	Removed int
	Msgs    []string
}

// EntryReducer transforms a list of permission entries by detecting
// and removing redundant entries. The reducer returns the reduced
// list and messages describing what was found. Mode control
// (warn vs reduce) is handled by the caller.
type EntryReducer interface {
	Reduce(ctx context.Context, entries []string) ([]string, *ReduceResult)
}

// bareToolNameRe matches a bare tool name without parentheses or prefix.
var bareToolNameRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

// isBareToolName reports whether entry is a bare tool name
// (e.g. "Bash", "Read") without specifier or mcp__ prefix.
func isBareToolName(entry string) bool {
	if strings.HasPrefix(entry, "mcp__") {
		return false
	}
	if strings.Contains(entry, "(") {
		return false
	}
	return bareToolNameRe.MatchString(entry)
}

// isBareMCPServer reports whether entry is a bare MCP server
// reference (e.g. "mcp__slack") that covers all tools for that server.
// Plugin entries (mcp__plugin_*) return false.
func isBareMCPServer(entry string) bool {
	if !strings.HasPrefix(entry, "mcp__") {
		return false
	}
	rest := entry[len("mcp__"):]
	if strings.HasPrefix(rest, "plugin_") {
		return false
	}
	// Bare server has no "__" after the server name and no "(".
	if strings.Contains(rest, "__") {
		return false
	}
	if strings.Contains(entry, "(") {
		return false
	}
	return rest != ""
}

// extractMCPServerFromEntry extracts the server name from an MCP
// entry string. Handles both bare entries and entries with parens.
// Returns ("", false) for non-MCP or plugin entries.
func extractMCPServerFromEntry(entry string) (string, bool) {
	// Strip paren suffix if present: "mcp__s__t(query)" → "mcp__s__t"
	raw := entry
	if idx := strings.IndexByte(raw, '('); idx >= 0 {
		raw = raw[:idx]
	}
	return extractMCPServerName(raw)
}

// DedupReducer removes exact duplicate entries from a permission list.
type DedupReducer struct{}

func (d *DedupReducer) Reduce(_ context.Context, entries []string) ([]string, *ReduceResult) {
	result := &ReduceResult{}
	if len(entries) == 0 {
		return entries, result
	}

	seen := make(map[string]int, len(entries))
	for _, e := range entries {
		seen[e]++
	}

	reduced := make([]string, 0, len(entries))
	emitted := make(set.Value[string], len(entries))
	for _, e := range entries {
		if emitted.Has(e) {
			continue
		}
		emitted.Add(e)
		reduced = append(reduced, e)
		if seen[e] > 1 {
			result.Msgs = append(result.Msgs,
				fmt.Sprintf("Duplicate: %q appears %d times", e, seen[e]))
			result.Removed += seen[e] - 1
		}
	}

	return reduced, result
}

// SubsumptionReducer removes entries that are subsumed by a broader
// entry. For example, if "Bash" (bare) is present, "Bash(git add *)"
// is redundant. If "mcp__github" (bare server) is present,
// "mcp__github__create_pr" is redundant.
type SubsumptionReducer struct{}

func (s *SubsumptionReducer) Reduce(_ context.Context, entries []string) ([]string, *ReduceResult) {
	result := &ReduceResult{}
	if len(entries) == 0 {
		return entries, result
	}

	// Collect bare tool names and bare MCP servers.
	bareTools := set.New[string]()
	bareMCPServers := set.New[string]()
	for _, e := range entries {
		if isBareToolName(e) {
			bareTools.Add(e)
		}
		if isBareMCPServer(e) {
			server := e[len("mcp__"):]
			bareMCPServers.Add(server)
		}
	}

	reduced := make([]string, 0, len(entries))
	for _, e := range entries {
		if reason := s.isSubsumed(e, bareTools, bareMCPServers); reason != "" {
			result.Msgs = append(result.Msgs,
				fmt.Sprintf("Redundant: %q is already covered by %s", e, reason))
			result.Removed++
			continue
		}
		reduced = append(reduced, e)
	}

	return reduced, result
}

// isSubsumed checks if entry is subsumed by a bare tool or bare MCP server.
// Returns the subsuming entry description, or "" if not subsumed.
func (s *SubsumptionReducer) isSubsumed(entry string, bareTools, bareMCPServers set.Value[string]) string {
	// Pattern 3: MCP entries subsumed by bare MCP server.
	// Check MCP first because mcp__server__tool(query) also
	// matches toolEntryRe.
	if strings.HasPrefix(entry, "mcp__") {
		server, ok := extractMCPServerFromEntry(entry)
		if !ok {
			return ""
		}
		if isBareMCPServer(entry) {
			return ""
		}
		if bareMCPServers.Has(server) {
			return fmt.Sprintf("%q", "mcp__"+server)
		}
		return ""
	}

	// Pattern 2: Tool(specifier) subsumed by bare Tool
	m := toolEntryRe.FindStringSubmatch(entry)
	if m != nil {
		toolName := m[1]
		if bareTools.Has(toolName) {
			return fmt.Sprintf("%q", toolName)
		}
	}

	return ""
}

// splitStringEntries separates string entries from non-string entries
// in a JSON array representation.
func splitStringEntries(arr []any) (strs []string, others []any) {
	for _, v := range arr {
		if s, ok := v.(string); ok {
			strs = append(strs, s)
		} else {
			others = append(others, v)
		}
	}
	return strs, others
}
