package tag

import (
	"fmt"
	"reflect"
	"strconv"
)

// ErrNotUsableAsTag is returned when a value cannot be used as a tag.
//
// It is also matchable as [ErrNotComparable] for backwards compatibility.
var ErrNotUsableAsTag errNotUsableAsTag

type errNotUsableAsTag struct {
	t             reflect.Type
	tagGetterPath string
	tagGetterType reflect.Type
}

func (e errNotUsableAsTag) Error() (s string) {
	if e.t != nil {
		s = e.t.String() + " is "
	}
	s += "not usable as tag"
	if e.tagGetterType != nil {
		return s + fmt.Sprintf("; found nested TagGetter at %s (%s); hint: implement JawsGetTag(tag.Context) on this type to delegate to that value, or pass that nested TagGetter directly", e.tagGetterPath, e.tagGetterType)
	}
	return s + "; found no nested TagGetter; hint: use a comparable tag value, or implement JawsGetTag(tag.Context) and return a comparable tag"
}

func (errNotUsableAsTag) Is(target error) bool {
	return target == ErrNotUsableAsTag || target == ErrNotComparable
}

// NewErrNotUsableAsTag returns [ErrNotUsableAsTag] if x cannot be used as a tag.
func NewErrNotUsableAsTag(x any) error {
	if err := NewErrNotComparable(x); err != nil {
		retErr := errNotUsableAsTag{t: reflect.TypeOf(x)}
		if path, tgType, ok := FindTagGetter(x); ok {
			retErr.tagGetterPath = path
			retErr.tagGetterType = tgType
		}
		return retErr
	}
	return nil
}

var tagGetterType = reflect.TypeFor[TagGetter]()

// maxHintScan is how many leading elements of an array or slice the
// [FindTagGetter] hint search inspects. It is intentionally small: the search
// only produces a human-readable diagnostic hint, so a nested TagGetter past
// this index simply will not be mentioned in the error message.
const maxHintScan = 4

// FindTagGetter searches x recursively for a nested [TagGetter].
//
// The search is bounded: it follows at most maxTagDepth levels of nesting and
// scans only the first maxHintScan elements of any array or slice. It is used
// only to enrich the [ErrNotUsableAsTag] diagnostic, so these bounds trade
// completeness for a cheap, terminating search.
func FindTagGetter(x any) (path string, tgType reflect.Type, found bool) {
	if x == nil {
		return
	}
	type seenPtr struct {
		t   reflect.Type
		ptr uintptr
	}
	seen := map[seenPtr]struct{}{}
	var walk func(v reflect.Value, currentPath string, depth int) bool
	walk = func(v reflect.Value, currentPath string, depth int) bool {
		if !v.IsValid() || depth > maxTagDepth {
			return false
		}
		t := v.Type()
		if t.Implements(tagGetterType) {
			path = currentPath
			if path == "" {
				path = "<value>"
			}
			tgType = t
			found = true
			return true
		}
		switch v.Kind() {
		case reflect.Interface:
			if v.IsNil() {
				return false
			}
			return walk(v.Elem(), currentPath, depth+1)
		case reflect.Pointer:
			if v.IsNil() {
				return false
			}
			p := seenPtr{t: t, ptr: v.Pointer()}
			if _, ok := seen[p]; ok {
				return false
			}
			seen[p] = struct{}{}
			return walk(v.Elem(), currentPath, depth+1)
		case reflect.Struct:
			for i := range t.NumField() {
				next := t.Field(i).Name
				if currentPath != "" {
					next = currentPath + "." + next
				}
				if walk(v.Field(i), next, depth+1) {
					return true
				}
			}
		case reflect.Array:
			n := min(v.Len(), maxHintScan)
			for i := range n {
				next := "[" + strconv.Itoa(i) + "]"
				if currentPath != "" {
					next = currentPath + next
				}
				if walk(v.Index(i), next, depth+1) {
					return true
				}
			}
		case reflect.Slice:
			if v.IsNil() {
				return false
			}
			p := seenPtr{t: t, ptr: v.Pointer()}
			if _, ok := seen[p]; ok {
				return false
			}
			seen[p] = struct{}{}
			n := min(v.Len(), maxHintScan)
			for i := range n {
				next := "[" + strconv.Itoa(i) + "]"
				if currentPath != "" {
					next = currentPath + next
				}
				if walk(v.Index(i), next, depth+1) {
					return true
				}
			}
		}
		return false
	}
	walk(reflect.ValueOf(x), path, 0)
	return
}
