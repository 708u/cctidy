package cctidy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// FormatResult holds the formatted output and statistics.
type FormatResult struct {
	Data  []byte
	Stats Summarizer
}

// Summarizer produces a human-readable summary of formatting results.
type Summarizer interface {
	Summary() string
}

// ClaudeJSONFormatterStats holds statistics for ~/.claude.json formatting.
type ClaudeJSONFormatterStats struct {
	ProjectsBefore int
	ProjectsAfter  int
	RepoBefore     int
	RepoAfter      int
	RemovedRepos   int
	SizeBefore     int
	SizeAfter      int
}

func (s *ClaudeJSONFormatterStats) Summary() string {
	var b strings.Builder
	if removed := s.ProjectsBefore - s.ProjectsAfter; removed > 0 {
		fmt.Fprintf(&b, "Projects: %d -> %d (removed %d)\n",
			s.ProjectsBefore, s.ProjectsAfter, removed)
	}
	if removed := s.RepoBefore - s.RepoAfter; removed > 0 {
		fmt.Fprintf(&b, "GitHub repo paths: %d -> %d (removed %d paths, %d empty repos)\n",
			s.RepoBefore, s.RepoAfter, removed, s.RemovedRepos)
	}
	fmt.Fprintf(&b, "Size: %s -> %s bytes\n",
		formatComma(int64(s.SizeBefore)), formatComma(int64(s.SizeAfter)))
	return b.String()
}

// SettingsJSONFormatterStats holds statistics for settings.json formatting.
type SettingsJSONFormatterStats struct {
	SizeBefore int
	SizeAfter  int
}

func (s *SettingsJSONFormatterStats) Summary() string {
	return fmt.Sprintf("Size: %s -> %s bytes\n",
		formatComma(int64(s.SizeBefore)), formatComma(int64(s.SizeAfter)))
}

// ClaudeJSONFormatter formats ~/.claude.json with path cleaning
// (removing non-existent projects and GitHub repo paths)
// in addition to key sorting and array sorting.
type ClaudeJSONFormatter struct {
	PathChecker PathChecker
}

func NewClaudeJSONFormatter(checker PathChecker) *ClaudeJSONFormatter {
	return &ClaudeJSONFormatter{PathChecker: checker}
}

// PathChecker checks whether a filesystem path exists.
type PathChecker interface {
	Exists(path string) bool
}

func (f *ClaudeJSONFormatter) Format(data []byte) (*FormatResult, error) {
	obj, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}

	stats := &ClaudeJSONFormatterStats{SizeBefore: len(data)}
	cj := &claudeJSONData{data: obj, checker: f.PathChecker}
	cj.cleanProjects(stats)
	cj.cleanGitHubRepoPaths(stats)
	sortArraysRecursive(cj.data)

	out, err := encodeJSON(cj.data)
	if err != nil {
		return nil, err
	}
	stats.SizeAfter = len(out)

	return &FormatResult{Data: out, Stats: stats}, nil
}

type claudeJSONData struct {
	data    map[string]any
	checker PathChecker
}

func (c *claudeJSONData) cleanProjects(stats *ClaudeJSONFormatterStats) {
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

func (c *claudeJSONData) cleanGitHubRepoPaths(stats *ClaudeJSONFormatterStats) {
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

// SettingsJSONFormatter formats settings.json / settings.local.json
// by sorting keys recursively and sorting homogeneous arrays.
// No path cleaning is performed.
type SettingsJSONFormatter struct{}

func NewSettingsJSONFormatter() *SettingsJSONFormatter { return &SettingsJSONFormatter{} }

func (s *SettingsJSONFormatter) Format(data []byte) (*FormatResult, error) {
	obj, err := decodeJSON(data)
	if err != nil {
		return nil, err
	}

	sortArraysRecursive(obj)

	out, err := encodeJSON(obj)
	if err != nil {
		return nil, err
	}

	return &FormatResult{
		Data:  out,
		Stats: &SettingsJSONFormatterStats{SizeBefore: len(data), SizeAfter: len(out)},
	}, nil
}

func decodeJSON(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return obj, nil
}

func encodeJSON(obj map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(obj); err != nil {
		return nil, fmt.Errorf("encoding JSON: %w", err)
	}
	return buf.Bytes(), nil
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
