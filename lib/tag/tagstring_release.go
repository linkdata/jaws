//go:build !debug && !race

package tag

// DebugRender reports whether [TagString] and [TagsString] render tags in full.
//
// It is false in the default (production) build, where they forward to the
// bounded, crash-safe [TagStringRelease]/[TagsStringRelease]. Build with -tags
// debug or -race to make it true.
const DebugRender = false

// TagString returns a debug string for tag.
//
// In the default build it forwards to the crash-safe [TagStringRelease]; see
// [DebugRender].
func TagString(tag any) string { return TagStringRelease(tag) }

// TagsString returns a debug string for a slice of tags.
//
// In the default build it forwards to the crash-safe [TagsStringRelease]; see
// [DebugRender].
func TagsString(tags []any) string { return TagsStringRelease(tags) }
