package cctidy

import (
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
	meta "go.abhg.dev/goldmark/frontmatter"
)

// fmParser is a reusable goldmark parser with frontmatter support.
var fmParser = goldmark.New(
	goldmark.WithExtensions(
		&meta.Extender{Mode: meta.SetMetadata},
	),
).Parser()

// AgentNameSet is a set of known agent names.
type AgentNameSet map[string]bool

// LoadAgentNames scans the agents directory and returns
// a set of agent names extracted from frontmatter.
// The frontmatter name field is the sole agent identifier;
// filenames are not used. Files without a valid name field
// are skipped.
// Returns an empty set if the directory does not exist.
func LoadAgentNames(dir string) AgentNameSet {
	set := make(AgentNameSet)
	if dir == "" {
		return set
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return set
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if fmName := parseAgentName(data); fmName != "" {
			set[fmName] = true
		}
	}
	return set
}

// parseAgentName extracts the name field from YAML
// frontmatter in data. Returns "" if not found.
func parseAgentName(data []byte) string {
	doc := fmParser.Parse(text.NewReader(data))
	m := doc.OwnerDocument().Meta()
	if m == nil {
		return ""
	}
	v, ok := m["name"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
