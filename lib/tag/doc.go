// Package tag expands and validates JaWS dependency tags used to associate Elements
// with application state and logical signals.
//
// Tags identify dependencies; they do not observe state or schedule work. Standard
// widgets register their source's tags during rendering.
// [github.com/linkdata/jaws.Request.Tag] and [github.com/linkdata/jaws.Element.Tag]
// provide manual registration. Pass tags to [github.com/linkdata/jaws.Request.Dirty]
// to schedule updates, or use them as [github.com/linkdata/jaws.Jaws.Broadcast]
// destinations.
//
// Registration is additive and lasts until the Element is removed or its Request
// ends. Adding a tag neither schedules an update nor changes targets already
// selected.
//
// Ordinary tags passed through [github.com/linkdata/jaws.Request.Dirty] match
// Elements across all live Requests. A non-nil pointer to
// [github.com/linkdata/jaws.Element] is the exact-target exception after expansion.
// [TagExpand] defines accepted tags and validation, [TagGetter] defines stable
// identity and concurrency requirements, and
// [github.com/linkdata/jaws.Request.TagsOf] reports the tags actually registered
// on an Element.
//
// The [tag guide] covers choosing dependencies and updating shared or local state.
//
// [tag guide]: https://github.com/linkdata/jaws/blob/main/doc/tags.md
package tag
