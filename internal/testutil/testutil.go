package testutil

import (
	"context"

	"github.com/708u/cctidy/internal/set"
)

// PathSet wraps a set.Value[string] and implements PathChecker.
type PathSet struct {
	s set.Value[string]
}

// Exists reports whether p is in the set.
func (ps PathSet) Exists(_ context.Context, p string) bool { return ps.s.Has(p) }

// CheckerFor returns a PathSet containing the given paths.
func CheckerFor(paths ...string) PathSet {
	return PathSet{s: set.New(paths...)}
}

// AllPathsExist is a PathChecker stub that reports all paths as existing.
type AllPathsExist struct{}

func (AllPathsExist) Exists(context.Context, string) bool { return true }

// NoPathsExist is a PathChecker stub that reports all paths as non-existent.
type NoPathsExist struct{}

func (NoPathsExist) Exists(context.Context, string) bool { return false }
