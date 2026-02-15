package cctidy

import (
	"os"
	"path/filepath"

	"github.com/708u/cctidy/internal/md"
	"github.com/708u/cctidy/internal/set"
)

// LoadAgentNames scans the agents directory and returns
// a set of agent names extracted from frontmatter.
// The frontmatter name field is the sole agent identifier;
// filenames are not used. Files without a valid name field
// are skipped.
// Returns an empty set if the directory does not exist.
func LoadAgentNames(dir string) set.Value[string] {
	s := set.New[string]()
	if dir == "" {
		return s
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return s
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
		if name := md.ParseName(data); name != "" {
			s.Add(name)
		}
	}
	return s
}
