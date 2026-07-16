package tag

import (
	"fmt"
	"html/template"
	"reflect"
	"runtime"
	"strings"

	"github.com/linkdata/jaws/lib/key"
)

// Tag is a simple comparable tag value.
type Tag string

// TagString returns a debug string for tag.
func TagString(tag any) string {
	if rv := reflect.ValueOf(tag); rv.IsValid() {
		if rv.Kind() == reflect.Pointer {
			return fmt.Sprintf("%T(%p)", tag, tag)
		}
		if stringer, ok := tag.(fmt.Stringer); ok {
			return fmt.Sprintf("%T(%s)", tag, stringer.String())
		}
	}
	return fmt.Sprintf("%#v", tag)
}

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
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		// For slices, Pointer() is the address of the first element, so two
		// distinct slices aliasing the same backing array at the same start
		// compare equal here. That can only over-detect a cycle and truncate
		// expansion early, which maxTagCount/maxTagDepth already bound; real tag
		// data does not take that shape.
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

func expand(depth int, ctx Context, tag any, result []any, active []any) ([]any, error) {
	if depth > maxTagDepth || len(result) > maxTagCount {
		return result, ErrTooManyTags
	}
	switch data := tag.(type) {
	case nil:
		return result, nil
	case Tag:
		return appendUniqueTag(result, tag)
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
		return expand(depth+1, ctx, data.JawsGetTag(ctx), result, append(active, data))
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
			if result, err = expand(depth+1, ctx, v, result, active); err != nil {
				return result, err
			}
		}
		return result, nil
	case string, template.HTML, template.HTMLAttr,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, key.Key,
		float32, float64, bool:
		return result, errIllegalTagType{tag: tag}
	default:
		return addTag(result, data)
	}
}

// TagExpand expands tag into a flat list of unique comparable tag values.
//
// tag may be nil, a [Tag], a slice of tags, a [TagGetter] or another comparable
// value. Primitive HTML/value types are rejected with [ErrIllegalTagType] to catch
// common accidental tags. A value that is not comparable at runtime or does not
// equal itself is rejected with [ErrNotUsableAsTag] (which also matches
// [ErrNotComparable] via [errors.Is]).
// Expansion that exceeds the nesting-depth or total-count limits is rejected with
// [ErrTooManyTags].
//
// On error, result holds the tags expanded before the failure; the exception is a
// value that is not usable as a map key, for which [ErrNotUsableAsTag] is returned
// with a nil result.
//
// Expansion reads tag and any values returned by [TagGetter.JawsGetTag] by
// reference, so tag and those values must not be mutated concurrently with the
// call.
func TagExpand(ctx Context, tag any) (result []any, err error) {
	// ensureUsableTag rejects tags that are not comparable at runtime, so the
	// existing == tag dedup in appendUniqueTag does not panic on them. recover
	// stays as a defense-in-depth net: should a non-comparable value ever reach
	// that comparison, recoverComparabilityPanic turns the specific "comparing
	// uncomparable type" runtime panic into [ErrNotUsableAsTag] and re-raises
	// anything else.
	defer func() {
		if r := recover(); r != nil {
			result, err = recoverComparabilityPanic(r, tag)
		}
	}()
	var activeArr [12]any
	return expand(0, ctx, tag, nil, activeArr[:0])
}

// recoverComparabilityPanic maps a panic recovered from tag expansion to a
// [TagExpand] result. A "comparing uncomparable type" runtime error — a value that
// passed the static comparability check but is not comparable at runtime — becomes
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

// MustTagExpand calls [TagExpand] and either logs or panics if expansion fails.
//
// On a non-nil ctx, expansion errors are passed to [Context.MustLog] (which logs
// them, or panics if no Logger is set); MustTagExpand then returns the partial
// result from [TagExpand]. A nil ctx always panics on error.
func MustTagExpand(ctx Context, tag any) []any {
	result, err := TagExpand(ctx, tag)
	if err != nil {
		if ctx != nil {
			ctx.MustLog(err)
		} else {
			panic(err)
		}
	}
	return result
}
