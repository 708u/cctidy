package ccfmt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

type FormatResult struct {
	Data  []byte
	Stats *Stats
}

type Stats struct {
	ProjectsBefore int
	ProjectsAfter  int
	RepoBefore     int
	RepoAfter      int
	RemovedRepos   int
	SizeBefore     int
	SizeAfter      int
}

func (s *Stats) ProjectsRemoved() int {
	return s.ProjectsBefore - s.ProjectsAfter
}

func (s *Stats) RepoPathsRemoved() int {
	return s.RepoBefore - s.RepoAfter
}

func (s *Stats) Summary(backupPath string) string {
	var b strings.Builder
	if s.ProjectsRemoved() > 0 {
		fmt.Fprintf(&b, "Projects: %d -> %d (removed %d)\n",
			s.ProjectsBefore, s.ProjectsAfter, s.ProjectsRemoved())
	}
	if s.RepoPathsRemoved() > 0 {
		fmt.Fprintf(&b, "GitHub repo paths: %d -> %d (removed %d paths, %d empty repos)\n",
			s.RepoBefore, s.RepoAfter, s.RepoPathsRemoved(), s.RemovedRepos)
	}
	fmt.Fprintf(&b, "Keys sorted recursively.\n")
	fmt.Fprintf(&b, "Size: %s -> %s bytes\n",
		formatComma(int64(s.SizeBefore)), formatComma(int64(s.SizeAfter)))
	if backupPath != "" {
		fmt.Fprintf(&b, "Backup: %s\n", backupPath)
	}
	return b.String()
}

type PathChecker interface {
	Exists(path string) bool
}

type Formatter struct {
	PathChecker PathChecker
}

func (f *Formatter) Format(data []byte) (*FormatResult, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	stats := &Stats{SizeBefore: len(data)}
	cj := &claudeJSON{data: obj, checker: f.PathChecker}
	cj.cleanProjects(stats)
	cj.cleanGitHubRepoPaths(stats)
	sortArraysRecursive(cj.data)

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cj.data); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}

	out := buf.Bytes()
	stats.SizeAfter = len(out)

	return &FormatResult{Data: out, Stats: stats}, nil
}

type claudeJSON struct {
	data    map[string]any
	checker PathChecker
}

func (c *claudeJSON) cleanProjects(stats *Stats) {
	raw, ok := c.data["projects"]
	if !ok {
		c.data["projects"] = map[string]any{}
		return
	}
	projects, ok := raw.(map[string]any)
	if !ok {
		return
	}

	stats.ProjectsBefore = len(projects)
	for p := range projects {
		if !c.checker.Exists(p) {
			delete(projects, p)
		}
	}
	stats.ProjectsAfter = len(projects)
}

func (c *claudeJSON) cleanGitHubRepoPaths(stats *Stats) {
	raw, ok := c.data["githubRepoPaths"]
	if !ok {
		c.data["githubRepoPaths"] = map[string]any{}
		return
	}
	repos, ok := raw.(map[string]any)
	if !ok {
		return
	}

	totalBefore := 0
	for _, v := range repos {
		paths, ok := v.([]any)
		if !ok {
			continue
		}
		totalBefore += len(paths)
	}
	stats.RepoBefore = totalBefore

	reposBefore := len(repos)
	for repo, v := range repos {
		paths, ok := v.([]any)
		if !ok {
			continue
		}
		var existing []any
		for _, p := range paths {
			s, ok := p.(string)
			if !ok {
				continue
			}
			if c.checker.Exists(s) {
				existing = append(existing, s)
			}
		}
		if len(existing) == 0 {
			delete(repos, repo)
		} else {
			repos[repo] = existing
		}
	}

	totalAfter := 0
	for _, v := range repos {
		paths, ok := v.([]any)
		if !ok {
			continue
		}
		totalAfter += len(paths)
	}
	stats.RepoAfter = totalAfter
	stats.RemovedRepos = reposBefore - len(repos)
}

func sortArraysRecursive(v any) {
	switch val := v.(type) {
	case map[string]any:
		for _, child := range val {
			sortArraysRecursive(child)
		}
	case []any:
		for _, child := range val {
			sortArraysRecursive(child)
		}
		sortHomogeneousArray(val)
	}
}

func sortHomogeneousArray(arr []any) {
	if len(arr) <= 1 {
		return
	}

	switch arr[0].(type) {
	case string:
		for _, v := range arr[1:] {
			if _, ok := v.(string); !ok {
				return
			}
		}
		slices.SortStableFunc(arr, func(a, b any) int {
			as := a.(string)
			bs := b.(string)
			if as < bs {
				return -1
			}
			if as > bs {
				return 1
			}
			return 0
		})
	case json.Number:
		for _, v := range arr[1:] {
			if _, ok := v.(json.Number); !ok {
				return
			}
		}
		slices.SortStableFunc(arr, func(a, b any) int {
			af, _ := a.(json.Number).Float64()
			bf, _ := b.(json.Number).Float64()
			if af < bf {
				return -1
			}
			if af > bf {
				return 1
			}
			return 0
		})
	case bool:
		for _, v := range arr[1:] {
			if _, ok := v.(bool); !ok {
				return
			}
		}
		slices.SortStableFunc(arr, func(a, b any) int {
			ab := a.(bool)
			bb := b.(bool)
			if ab == bb {
				return 0
			}
			if !ab {
				return -1
			}
			return 1
		})
	}
}

func formatComma(n int64) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}

	var b strings.Builder
	start := len(s) % 3
	if start > 0 {
		b.WriteString(s[:start])
	}
	for i := start; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
