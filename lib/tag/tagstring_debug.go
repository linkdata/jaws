//go:build debug || race

package tag

// DebugRender reports whether [TagString] and [TagsString] render tags in full.
//
// It is true in the development build (-tags debug or -race), where they forward
// to the full-detail [TagStringDebug]/[TagsStringDebug]. The default build makes
// it false and forwards to the crash-safe release variants instead.
const DebugRender = true

// TagString returns a debug string for tag.
//
// This is the development build, so it forwards to the full-detail
// [TagStringDebug], which can overflow the stack or exhaust memory on a
// self-referential or oversized tag; see [DebugRender].
func TagString(tag any) string { return TagStringDebug(tag) }

// TagsString returns a debug string for a slice of tags.
//
// This is the development build, so it forwards to the full-detail, unbounded
// [TagsStringDebug]; see [DebugRender].
func TagsString(tags []any) string { return TagsStringDebug(tags) }
