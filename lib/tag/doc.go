// Package tag expands JaWS dependency tags into keys used to associate Elements
// with application state and logical signals.
//
// Tags identify dependencies; they do not observe state or schedule work. Register
// them with [github.com/linkdata/jaws.Request.Tag] or
// [github.com/linkdata/jaws.Element.Tag], mutate the authoritative state, then pass
// the corresponding keys to [github.com/linkdata/jaws.Request.Dirty] or use them as
// [github.com/linkdata/jaws.Jaws.Broadcast] destinations.
//
// # Choosing tags
//
// Prefer stable identities derived from the data being rendered. Use a pointer
// when output depends on an object as a whole, a distinct comparable wrapper type
// for an independently changing part, or a named empty struct for a shared signal.
// Name a tag for the dependency, not the event that changes it. Plain strings are
// not tag keys; use [Tag] for a string-named logical signal.
//
// Register an item with its own key and a separate group key when both narrow and
// broad updates are needed. Do not include the group key in the item's
// [TagGetter.JawsGetTag] result: dirtying the item would then expand to the group
// and refresh every group listener.
//
// # Registration and targeting
//
// Registration is additive and lasts until the Element is removed or its Request
// ends. Adding a tag neither schedules an update nor changes targets already
// selected. Register known dependencies during initial rendering; add a
// later-discovered dependency only when it remains valid for the Element's
// remaining lifetime.
//
// Ordinary keys passed through [github.com/linkdata/jaws.Request.Dirty] match
// Elements across all live Requests. A non-nil pointer to
// [github.com/linkdata/jaws.Element] is the exact-target exception after expansion.
// [TagExpand] defines accepted keys and validation, [TagGetter] defines stable
// identity and concurrency requirements, and
// [github.com/linkdata/jaws.Request.TagsOf] reports the keys actually registered
// on an Element.
package tag
