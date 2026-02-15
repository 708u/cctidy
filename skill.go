package cctidy

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/708u/cctidy/internal/set"
)

// LoadSkillNames scans the skills and commands directories under
// claudeDir and returns a set of skill names.
//
// Skills are identified by subdirectories in <claudeDir>/skills/
// that contain a SKILL.md file. The subdirectory name is the skill name.
//
// Commands are identified by .md files in <claudeDir>/commands/.
// The filename without extension is the skill name.
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
func loadSkillsDir(dir string, s set.Value[string]) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skillFile := filepath.Join(dir, e.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); err == nil {
			s.Add(e.Name())
		}
	}
}

// loadCommandsDir scans dir for .md files.
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
		if name != "" {
			s.Add(name)
		}
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
	specifier := entry.Specifier
	// Plugin skills use "plugin:name" convention
	// and are managed by the plugin system.
	if strings.Contains(specifier, ":") {
		return ToolSweepResult{}
	}
	// Extract name from specifier (e.g. "name *" -> "name").
	name, _, _ := strings.Cut(specifier, " ")
	if s.skills.Len() == 0 {
		return ToolSweepResult{}
	}
	if s.skills.Has(name) {
		return ToolSweepResult{}
	}
	return ToolSweepResult{Sweep: true}
}
