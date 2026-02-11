package cctidy

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
)

var absPathRe = regexp.MustCompile(`(?:^|[\s=(])(/[^\s"'):*]+)`)
var relPathRe = regexp.MustCompile(`\./[^\s"'):*]+`)

// ExtractAbsolutePaths extracts all absolute paths from a permission entry string.
func ExtractAbsolutePaths(entry string) []string {
	matches := absPathRe.FindAllStringSubmatch(entry, -1)
	if matches == nil {
		return nil
	}
	paths := make([]string, len(matches))
	for i, m := range matches {
		paths[i] = m[1]
	}
	return paths
}

// ExtractRelativePaths extracts all relative paths from a permission entry string.
func ExtractRelativePaths(entry string) []string {
	matches := relPathRe.FindAllString(entry, -1)
	if matches == nil {
		return nil
	}
	return matches
}

// PruneResult holds statistics from permission pruning.
type PruneResult struct {
	PrunedAllow   int
	PrunedDeny    int
	PrunedAsk     int
	RelativeWarns []string
}

func prunePermissions(ctx context.Context, obj map[string]any, checker PathChecker, baseDir string) *PruneResult {
	result := &PruneResult{}

	raw, ok := obj["permissions"]
	if !ok {
		return result
	}
	perms, ok := raw.(map[string]any)
	if !ok {
		return result
	}

	type category struct {
		key   string
		count *int
	}
	categories := []category{
		{"allow", &result.PrunedAllow},
		{"deny", &result.PrunedDeny},
		{"ask", &result.PrunedAsk},
	}

	for _, cat := range categories {
		raw, ok := perms[cat.key]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}

		kept := make([]any, 0, len(arr))
		for _, v := range arr {
			entry, ok := v.(string)
			if !ok {
				kept = append(kept, v)
				continue
			}

			if shouldPrune(ctx, entry, checker, baseDir, result) {
				*cat.count++
				continue
			}

			kept = append(kept, v)
		}
		perms[cat.key] = kept
	}

	return result
}

func shouldPrune(ctx context.Context, entry string, checker PathChecker, baseDir string, result *PruneResult) bool {
	absPaths := ExtractAbsolutePaths(entry)
	if len(absPaths) > 0 {
		return !slices.ContainsFunc(absPaths, func(p string) bool {
			return checker.Exists(ctx, p)
		})
	}

	relPaths := ExtractRelativePaths(entry)
	if len(relPaths) > 0 {
		if baseDir == "" {
			result.RelativeWarns = append(result.RelativeWarns, entry)
			return false
		}
		return !slices.ContainsFunc(relPaths, func(p string) bool {
			return checker.Exists(ctx, filepath.Join(baseDir, p))
		})
	}

	return false
}
