package cctidy

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/text"
	meta "go.abhg.dev/goldmark/frontmatter"
)

// AgentNameSet is a set of known agent names.
type AgentNameSet map[string]bool

// LoadAgentNames scans the agents directory and returns
// a set of agent names. Both filename-based names and
// frontmatter name fields are included.
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
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		baseName := strings.TrimSuffix(name, ".md")
		set[baseName] = true

		data, err := os.ReadFile(filepath.Join(dir, name))
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
	md := goldmark.New(
		goldmark.WithExtensions(
			&meta.Extender{Mode: meta.SetMetadata},
		),
	)
	doc := md.Parser().Parse(text.NewReader(data))
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
