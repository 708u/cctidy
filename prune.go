package cctidy

import (
	"context"
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

// ContainsRelativePath reports whether the entry contains a relative path.
func ContainsRelativePath(entry string) bool {
	return relPathRe.MatchString(entry)
}

// PruneResult holds statistics from permission pruning.
type PruneResult struct {
	PrunedAllow   int
	PrunedDeny    int
	PrunedAsk     int
	RelativeWarns []string
}

func prunePermissions(ctx context.Context, obj map[string]any, checker PathChecker) *PruneResult {
	result := &PruneResult{}
	if checker == nil {
		return result
	}

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

			if ContainsRelativePath(entry) {
				result.RelativeWarns = append(result.RelativeWarns, entry)
			}

			absPaths := ExtractAbsolutePaths(entry)
			if len(absPaths) > 0 {
				if !slices.ContainsFunc(absPaths, func(p string) bool {
					return checker.Exists(ctx, p)
				}) {
					*cat.count++
					continue
				}
			}

			kept = append(kept, v)
		}
		perms[cat.key] = kept
	}

	return result
}
