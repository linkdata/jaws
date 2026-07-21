package tag

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

// maxTagString bounds a rendered tag in the release build.
//
// A single tag's type name is truncated to this many bytes, and a tag list stops
// appending once it reaches this size (so its output stays under roughly twice
// this, one element being allowed to complete). Ordinary tags are far shorter, so
// the cap only affects pathological input such as a reflect-built type with a
// huge name.
const maxTagString = 4096

// truncMarker is appended where release rendering truncates.
const truncMarker = "…"

// TagStringRelease renders tag showing only its type, plus its address when tag
// is a pointer whose address can be read without panicking.
//
// It reads the type via reflection, never hands tag itself to a formatting verb,
// and never invokes a String/GoString/Format/Error method or touches the
// pointed-to memory; the address is read under a recover (see pointerAddr)
// because [reflect.Value.Pointer] can panic on some cgo / not-in-heap pointers,
// and an over-long type name is truncated. It therefore cannot recurse, overflow
// the stack, exhaust memory, or panic no matter what tag contains — at the cost
// of not showing the tag's contents.
//
// [TagString] forwards here in the default build.
func TagStringRelease(tag any) string {
	rv := reflect.ValueOf(tag)
	if !rv.IsValid() {
		return "<nil>"
	}
	s := clipTagString(rv.Type().String())
	if rv.Kind() == reflect.Pointer {
		if addr, ok := pointerAddr(rv); ok {
			return s + fmt.Sprintf("(0x%x)", addr)
		}
	}
	return s
}

// pointerAddr returns rv's address as a uintptr.
//
// [reflect.Value.Pointer] panics on some cgo / not-in-heap pointers (and on a
// non-pointer kind), so pointerAddr recovers and reports ok=false, letting release
// rendering fall back to type-only for such a tag instead of crashing.
func pointerAddr(rv reflect.Value) (addr uintptr, ok bool) {
	defer func() {
		if recover() != nil {
			addr, ok = 0, false
		}
	}()
	return rv.Pointer(), true
}

// clipTagString truncates s to maxTagString bytes on a UTF-8 boundary, appending
// truncMarker, so a pathologically long type name cannot produce unbounded output.
func clipTagString(s string) string {
	if len(s) <= maxTagString {
		return s
	}
	n := maxTagString
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + truncMarker
}

// TagStringDebug renders tag in full: a pointer as its type and address, a
// [fmt.Stringer] as its type and String(), and every other value through "%#v".
//
// It is the most informative form, but because it descends into the value it can
// overflow the stack or exhaust memory on a self-referential or oversized tag, so
// it is unsuitable for untrusted input.
//
// [TagString] forwards here in the debug and -race builds.
func TagStringDebug(tag any) string {
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

// TagsStringRelease renders a slice of tags, each with [TagStringRelease].
//
// It stops once the output reaches maxTagString, so it inherits the per-element
// crash-safety and stays bounded however long the slice is. [TagsString] forwards
// here in the default build.
func TagsStringRelease(tags []any) string {
	return tagsString(tags, TagStringRelease, maxTagString)
}

// TagsStringDebug renders a slice of tags in full, each with [TagStringDebug].
//
// It applies no aggregate size limit, matching the informative (unsafe) debug
// contract. [TagsString] forwards here in the debug and -race builds.
func TagsStringDebug(tags []any) string {
	return tagsString(tags, TagStringDebug, 0)
}

// tagsString renders tags as "[e0 e1 …]" using one to format each element.
//
// The slice itself is never handed to fmt, so the rendering inherits the
// per-element formatter's crash-safety. When limit > 0 it stops once the output
// reaches limit bytes, so an enormous slice cannot produce unbounded output;
// limit <= 0 renders every element.
func tagsString(tags []any, one func(any) string, limit int) string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, t := range tags {
		if limit > 0 && sb.Len() >= limit {
			sb.WriteString(truncMarker)
			break
		}
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(one(t))
	}
	sb.WriteByte(']')
	return sb.String()
}
