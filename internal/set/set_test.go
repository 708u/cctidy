package set_test

import (
	"testing"

	"github.com/708u/cctidy/internal/set"
)

func TestNew(t *testing.T) {
	t.Parallel()

	s := set.New("a", "b", "c")

	if s.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", s.Len())
	}
	for _, e := range []string{"a", "b", "c"} {
		if !s.Has(e) {
			t.Errorf("Has(%q) = false, want true", e)
		}
	}
}

func TestNewEmpty(t *testing.T) {
	t.Parallel()

	s := set.New[string]()

	if s.Len() != 0 {
		t.Fatalf("Len() = %d, want 0", s.Len())
	}
}

func TestNewDeduplicates(t *testing.T) {
	t.Parallel()

	s := set.New("x", "x", "y")

	if s.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", s.Len())
	}
}

func TestAdd(t *testing.T) {
	t.Parallel()

	s := set.New[int]()
	s.Add(1)

	if !s.Has(1) {
		t.Fatal("Has(1) = false after Add(1)")
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}

	s.Add(1)
	if s.Len() != 1 {
		t.Fatalf("Len() = %d after duplicate Add, want 1", s.Len())
	}
}

func TestDelete(t *testing.T) {
	t.Parallel()

	s := set.New("a", "b")
	s.Delete("a")

	if s.Has("a") {
		t.Fatal("Has(\"a\") = true after Delete")
	}
	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestDeleteNonExistent(t *testing.T) {
	t.Parallel()

	s := set.New("a")
	s.Delete("z")

	if s.Len() != 1 {
		t.Fatalf("Len() = %d, want 1", s.Len())
	}
}

func TestHasOnEmpty(t *testing.T) {
	t.Parallel()

	s := set.New[string]()

	if s.Has("x") {
		t.Fatal("Has on empty set returned true")
	}
}
