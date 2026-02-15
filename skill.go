package cctidy

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/708u/cctidy/internal/md"
	"github.com/708u/cctidy/internal/set"
)

// LoadSkillNames scans the skills and commands directories under
// claudeDir and returns a set of skill names.
//
// Skills are identified by subdirectories in <claudeDir>/skills/
// that contain a SKILL.md file. If SKILL.md has a frontmatter
// name field, that name is used; otherwise the directory name
// is used.
//
// Commands are identified by .md files in <claudeDir>/commands/.
// If a file has a frontmatter name field, that name is used;
// otherwise the filename without extension is used.
//
// Returns an empty set if claudeDir is empty or unreadable.
func LoadSkillNames(claudeDir string) set.Value[string] {
	s := set.New[string]()
	if claudeDir == "" {
		return s
	}
	loadSkillsDir(filepath.Join(claudeDir, "skills"), s)
	loadCommandsDir(filepath.Join(claudeDir, "commands"), s)
	return s
}

// loadSkillsDir scans dir for subdirectories containing SKILL.md.
// If SKILL.md has a frontmatter name field, that name is used;
// otherwise the directory name is used.
func loadSkillsDir(dir string, s set.Value[string]) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		addSkillName(s, filepath.Join(dir, e.Name(), "SKILL.md"), e.Name())
	}
}

// loadCommandsDir scans dir for .md files.
// If a file has a frontmatter name field, that name is used;
// otherwise the filename without extension is used.
func loadCommandsDir(dir string, s set.Value[string]) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".md")
		if name == "" {
			continue
		}
		addSkillName(s, filepath.Join(dir, e.Name()), name)
	}
}

// addSkillName reads a markdown file and adds the frontmatter name
// to the set. Falls back to fallback when the file is
// unreadable or has no name field.
func addSkillName(s set.Value[string], path, fallback string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if name := md.ParseName(data); name != "" {
		s.Add(name)
	} else {
		s.Add(fallback)
	}
}

// SkillToolSweeper sweeps Skill permission entries where the
// referenced skill or command no longer exists. Plugin skills
// (containing ":") are always kept.
type SkillToolSweeper struct {
	skills set.Value[string]
}

// NewSkillToolSweeper creates a SkillToolSweeper.
func NewSkillToolSweeper(skills set.Value[string]) *SkillToolSweeper {
	return &SkillToolSweeper{skills: skills}
}

func (s *SkillToolSweeper) ShouldSweep(_ context.Context, entry StandardEntry) ToolSweepResult {
	if s.skills.Len() == 0 {
		return ToolSweepResult{}
	}
	specifier := entry.Specifier
	// Plugin skills use "plugin:name" convention
	// and are managed by the plugin system.
	if strings.Contains(specifier, ":") {
		return ToolSweepResult{}
	}
	// Extract name from specifier (e.g. "name *" -> "name").
	name, _, _ := strings.Cut(specifier, " ")
	if s.skills.Has(name) {
		return ToolSweepResult{}
	}
	return ToolSweepResult{Sweep: true}
}
