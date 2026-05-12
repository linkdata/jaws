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
		return s + fmt.Sprintf("; found nested TagGetter at %s (%s); hint: implement JawsGetTag(jawstags.Context) on this type to delegate to that value, or pass that nested TagGetter directly", e.tagGetterPath, e.tagGetterType)
	}
	return s + "; found no nested TagGetter; hint: use a comparable tag value, or implement JawsGetTag(jawstags.Context) and return a comparable tag"
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

// FindTagGetter searches x recursively for a nested [TagGetter].
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
		if !v.IsValid() || depth > 10 {
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
			n := min(v.Len(), 4)
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
			n := min(v.Len(), 4)
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
