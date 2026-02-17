package cctidy

import (
	"encoding/json"
	"os"
	"strings"
)

// EnabledPlugins holds the enabled state of marketplace plugins
// collected from settings files. A nil receiver means no
// enabledPlugins data was found in any file (sweeper inactive,
// all plugin entries kept).
type EnabledPlugins struct {
	names map[string]bool // plugin name → any enabled?
}

// IsEnabled reports whether the plugin is enabled.
// A nil receiver returns true (conservative: keep all).
// A name not present in the map returns true (unknown plugin,
// conservative: keep). Only returns false when the name is
// explicitly registered as disabled.
func (ep *EnabledPlugins) IsEnabled(name string) bool {
	if ep == nil {
		return true
	}
	enabled, known := ep.names[name]
	if !known {
		return true
	}
	return enabled
}

// LoadEnabledPlugins reads enabledPlugins from the given settings
// file paths and returns an aggregated result.
// Returns nil when no file contains an enabledPlugins key.
// JSON parse errors in individual files are silently ignored
// (treated as absent).
func LoadEnabledPlugins(paths ...string) *EnabledPlugins {
	var found bool
	names := make(map[string]bool)

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			continue
		}
		raw, ok := obj["enabledPlugins"]
		if !ok {
			continue
		}
		var plugins map[string]bool
		if err := json.Unmarshal(raw, &plugins); err != nil {
			continue
		}
		found = true
		for key, enabled := range plugins {
			pluginName := extractPluginNameFromKey(key)
			if pluginName == "" {
				continue
			}
			// OR merge: any true → true
			if enabled {
				names[pluginName] = true
			} else if !names[pluginName] {
				names[pluginName] = false
			}
		}
	}

	if !found {
		return nil
	}
	return &EnabledPlugins{names: names}
}

// extractPluginNameFromKey extracts the plugin name from an
// enabledPlugins key. Keys have the form "name@marketplace".
// Returns the part before @, or empty if no @ is present.
func extractPluginNameFromKey(key string) string {
	idx := strings.IndexByte(key, '@')
	if idx <= 0 {
		return ""
	}
	return key[:idx]
}

// extractPluginNameFromEntry extracts the plugin name from a
// permission entry string. Returns ("", false) for non-plugin
// entries.
//
// Supported patterns:
//   - mcp__plugin_<name>_<server>__<tool> → first _ token of segment
//   - Tool(plugin-name:suffix) → part before :
//   - mcp__plugin_<name>_<server> (bare) → first _ token of segment
func extractPluginNameFromEntry(entry string) (string, bool) {
	// MCP plugin entries: mcp__plugin_*
	if strings.HasPrefix(entry, "mcp__plugin_") {
		return extractMCPPluginName(entry)
	}

	// Standard entries: Tool(specifier) where specifier contains ":"
	m := toolEntryRe.FindStringSubmatch(entry)
	if m != nil {
		specifier := m[2]
		// Extract name before ":" for plugin entries.
		// Also handle "name * " prefix forms by cutting at space first.
		name, _, _ := strings.Cut(specifier, " ")
		if idx := strings.IndexByte(name, ':'); idx > 0 {
			return name[:idx], true
		}
		return "", false
	}

	// Bare mcp__plugin_ entries without parens
	// (already handled above by the HasPrefix check)
	return "", false
}

// extractMCPPluginName extracts the plugin name from an MCP plugin
// tool name. The format is mcp__plugin_<name>_<server>__<tool>
// or mcp__plugin_<name>_<server> (bare).
// Returns the first _ token of the <name>_<server> segment.
func extractMCPPluginName(entry string) (string, bool) {
	// Strip "mcp__plugin_" prefix
	rest := entry[len("mcp__plugin_"):]
	// Remove trailing (args) if present
	if idx := strings.IndexByte(rest, '('); idx >= 0 {
		rest = rest[:idx]
	}
	// Remove __tool suffix if present
	if idx := strings.Index(rest, "__"); idx > 0 {
		rest = rest[:idx]
	}
	// rest is now "<name>_<server>" — take first _ token as plugin name
	if rest == "" {
		return "", false
	}
	name, _, _ := strings.Cut(rest, "_")
	if name == "" {
		return "", false
	}
	return name, true
}
