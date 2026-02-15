package set

// Value is a generic set backed by a map with empty struct values.
type Value[E comparable] map[E]struct{}

// New creates a set containing the given elements.
func New[E comparable](elems ...E) Value[E] {
	s := make(Value[E], len(elems))
	for _, e := range elems {
		s[e] = struct{}{}
	}
	return s
}

// Add inserts an element into the set.
func (v Value[E]) Add(e E) {
	v[e] = struct{}{}
}

// Delete removes an element from the set.
func (v Value[E]) Delete(e E) {
	delete(v, e)
}

// Has reports whether the set contains the element.
func (v Value[E]) Has(e E) bool {
	_, ok := v[e]
	return ok
}

// Len returns the number of elements in the set.
func (v Value[E]) Len() int {
	return len(v)
}
