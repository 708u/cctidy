package set

// Set is a generic set backed by a map with empty struct values.
type Set[E comparable] map[E]struct{}

// New creates a Set containing the given elements.
func New[E comparable](elems ...E) Set[E] {
	s := make(Set[E], len(elems))
	for _, e := range elems {
		s[e] = struct{}{}
	}
	return s
}

// Add inserts an element into the set.
func (s Set[E]) Add(e E) {
	s[e] = struct{}{}
}

// Delete removes an element from the set.
func (s Set[E]) Delete(e E) {
	delete(s, e)
}

// Has reports whether the set contains the element.
func (s Set[E]) Has(e E) bool {
	_, ok := s[e]
	return ok
}

// Len returns the number of elements in the set.
func (s Set[E]) Len() int {
	return len(s)
}
