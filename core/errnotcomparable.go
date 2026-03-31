package jaws

import "github.com/linkdata/jaws/core/tags"

// ErrNotComparable is returned when a UI object or tag is not comparable.
var ErrNotComparable = tags.ErrNotComparable

func newErrNotComparable(x any) error {
	return tags.NewErrNotComparable(x)
}
