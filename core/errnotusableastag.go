package jaws

import (
	"reflect"

	"github.com/linkdata/jaws/core/tags"
)

// ErrNotUsableAsTag is returned when a value cannot be used as a tag.
//
// It is also matchable as ErrNotComparable for backwards compatibility.
var ErrNotUsableAsTag = tags.ErrNotUsableAsTag

func newErrNotUsableAsTag(x any) error {
	return tags.NewErrNotUsableAsTag(x)
}

func findTagGetter(x any) (path string, tgType reflect.Type, found bool) {
	return tags.FindTagGetter(x)
}
