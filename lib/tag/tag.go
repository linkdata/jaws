package tag

import (
	"html/template"
	"reflect"
	"runtime"
	"strings"

	"github.com/linkdata/jaws/lib/jid"
	"github.com/linkdata/jaws/lib/key"
)

// Tag is a simple comparable tag value.
type Tag string

// Expansion limits guarding against runaway recursion or pathological input.
const (
	// maxTagDepth is the maximum [TagGetter]/slice nesting depth that tag
	// expansion (and the [FindTagGetter] hint search) will follow.
	maxTagDepth = 10
	// maxTagCount is the maximum number of unique tags a single expansion may
	// produce before returning [ErrTooManyTags].
	maxTagCount = 100
)

func ensureUsableTag(tag any) error {
	if usableAsTag(tag) {
		return nil
	}
	return newErrNotUsableAsTag(tag)
}

// usableAsTag reports whether tag is non-nil, comparable, and equal to itself.
func usableAsTag(tag any) (ok bool) {
	if tag != nil {
		// Interface equality panics when the dynamic value is not comparable. If it
		// does, recover leaves the named result at its false zero value.
		defer func() { _ = recover() }()
		other := tag
		ok = tag == other
	}
	return
}

func appendUniqueTag(result []any, tag any) ([]any, error) {
	for _, existing := range result {
		if existing == tag {
			return result, nil
		}
	}
	result = append(result, tag)
	// maxTagCount is an inclusive soft cap: the over-limit tag is appended before
	// the check, so the partial result returned alongside ErrTooManyTags may hold
	// one element more than maxTagCount. Callers either log or panic on the error.
	if len(result) > maxTagCount {
		return result, ErrTooManyTags
	}
	return result, nil
}

func addTag(result []any, tag any) ([]any, error) {
	if err := ensureUsableTag(tag); err != nil {
		return nil, err
	}
	return appendUniqueTag(result, tag)
}

func sameActiveNode(a, b any) bool {
	if a == nil || b == nil {
		return a == b
	}
	ta := reflect.TypeOf(a)
	if ta != reflect.TypeOf(b) {
		return false
	}
	if ta.Comparable() {
		return a == b
	}
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)
	switch va.Kind() {
	case reflect.Func:
		// Value.Pointer identifies shared function code, not closure identity, so it
		// cannot detect cycles. maxTagDepth still bounds recursive function getters.
		return false
	case reflect.Slice:
		// Value.Pointer for a slice is the address of its first element, so two
		// distinct views into the same backing array share it. A slice header is
		// fully identified by its pointer, length and capacity, so compare all
		// three: views differing in length or capacity are distinct nodes (a named
		// slice TagGetter can observe its capacity via cap and yield different
		// tags), while a genuine self-referential slice re-enters with an identical
		// header and is still detected as a cycle.
		return va.Pointer() == vb.Pointer() && va.Len() == vb.Len() && va.Cap() == vb.Cap()
	case reflect.Pointer, reflect.Map, reflect.Chan, reflect.UnsafePointer:
		return va.Pointer() == vb.Pointer()
	default:
		return false
	}
}

func findActiveIndex(active []any, tag any) int {
	for i := len(active) - 1; i >= 0; i-- {
		if sameActiveNode(active[i], tag) {
			return i
		}
	}
	return -1
}

// addActiveTags closes a detected expansion cycle by re-emitting the TagGetter
// members of the active chain from the revisited node onward. Slice frames in
// the chain are intentionally skipped: a slice is not itself a tag, only its
// elements are. So a cyclic TagGetter graph (e.g. mutual a<->b taggers) resolves
// to the set of taggers participating in the cycle.
func addActiveTags(result []any, active []any) ([]any, error) {
	var err error
	for _, node := range active {
		if _, ok := node.(TagGetter); ok {
			if result, err = addTag(result, node); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

func hasNonNilTag(tags []any) bool {
	for _, tag := range tags {
		if tag != nil {
			return true
		}
	}
	return false
}

func expand(depth int, tagValue any, result []any, active []any) ([]any, error) {
	if depth > maxTagDepth || len(result) > maxTagCount {
		return result, ErrTooManyTags
	}
	switch data := tagValue.(type) {
	case nil:
		return result, nil
	case Tag:
		return appendUniqueTag(result, tagValue)
	case []Tag:
		if result == nil && len(data) > 0 {
			result = make([]any, 0, len(data))
		}
		var err error
		for _, v := range data {
			if result, err = appendUniqueTag(result, v); err != nil {
				return result, err
			}
		}
		return result, nil
	case TagGetter:
		if idx := findActiveIndex(active, data); idx >= 0 {
			return addActiveTags(result, active[idx:])
		}
		return expand(depth+1, data.JawsGetTag(), result, append(active, data))
	case []any:
		if !hasNonNilTag(data) {
			return result, nil
		}
		if idx := findActiveIndex(active, data); idx >= 0 {
			return addActiveTags(result, active[idx:])
		}
		if result == nil && len(data) > 0 {
			result = make([]any, 0, len(data))
		}
		active = append(active, data)
		var err error
		for _, v := range data {
			if result, err = expand(depth+1, v, result, active); err != nil {
				return result, err
			}
		}
		return result, nil
	case string, template.HTML, template.HTMLAttr, jid.Jid,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, key.Key,
		float32, float64, bool:
		// Reject these exact types to catch common accidental tags while still
		// allowing named domain types with the same underlying representation.
		return result, errIllegalTagType{tag: tagValue}
	default:
		return addTag(result, data)
	}
}

// TagExpand expands tagValue into a flat list of unique, usable tag keys.
//
// tagValue may be nil, a [Tag], []Tag, []any, a [TagGetter], or another value that is
// comparable at runtime and equals itself. A nil interface contributes no keys. A
// typed nil is a non-nil interface and follows the normal rules for its dynamic type,
// including dispatch to [TagGetter.JawsGetTag] when it implements [TagGetter].
//
// The predeclared string, bool, signed integer, unsigned integer other than uintptr,
// and floating-point types are rejected with [ErrIllegalTagType], as are
// [template.HTML], [template.HTMLAttr], [jid.Jid] and [key.Key]. An unusable expanded
// key is rejected with [ErrNotUsableAsTag], which also matches [ErrNotComparable]
// under errors.Is. Expansion that exceeds the nesting-depth or total-count limits is
// rejected with [ErrTooManyTags].
//
// On error, result contains the tags expanded before the failure. If an expanded
// value is not usable as a tag key, result is nil and err matches
// [ErrNotUsableAsTag].
//
// A single call may invoke a [TagGetter] more than once, and later calls may expand
// the same value again. When expansion encounters a cyclic TagGetter graph, it uses
// the participating TagGetter values themselves as keys; they must therefore be
// usable as tags. Implementations must satisfy [TagGetter]'s stable-identity contract.
//
// TagExpand does not copy input slices or slices returned by
// [TagGetter.JawsGetTag] before traversing them. They must not be mutated concurrently
// with expansion.
//
// Errors are returned rather than logged. Use
// [github.com/linkdata/jaws.Jaws.MustTagExpand] to report them through a configured
// logger instead.
func TagExpand(tagValue any) (result []any, err error) {
	// ensureUsableTag rejects tags that are not comparable at runtime, so the
	// existing == tag dedup in appendUniqueTag does not panic on them. recover
	// stays as a defense-in-depth net: should a non-comparable value ever reach
	// that comparison, recoverComparabilityPanic turns the specific "comparing
	// uncomparable type" runtime panic into [ErrNotUsableAsTag] and re-raises
	// anything else.
	defer func() {
		if r := recover(); r != nil {
			result, err = recoverComparabilityPanic(r, tagValue)
		}
	}()
	var activeArr [12]any
	return expand(0, tagValue, nil, activeArr[:0])
}

// recoverComparabilityPanic maps a panic recovered from tag expansion to a
// [TagExpand] result. A "comparing uncomparable type" runtime error becomes
// [ErrNotUsableAsTag] with a nil result; any other panic value is re-raised.
func recoverComparabilityPanic(r any, tag any) (result []any, err error) {
	if re, ok := r.(runtime.Error); ok && strings.Contains(re.Error(), "comparing uncomparable type") {
		if err = NewErrNotUsableAsTag(tag); err == nil {
			// The top-level tag is comparable; a nested element panicked.
			err = ErrNotUsableAsTag
		}
		return nil, err
	}
	panic(r)
}
