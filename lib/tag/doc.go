// Package tag expands JaWS tag values into keys used to associate elements with
// application dependencies.
//
// Tags associate [github.com/linkdata/jaws.Element] values with the application
// state and logical signals on which their output or tag-addressed behavior
// depends. After changing state, applications pass the corresponding keys to
// [github.com/linkdata/jaws.Request.Dirty] to schedule updates of matching Elements
// across live Requests, or use them as destinations for
// [github.com/linkdata/jaws.Jaws.Broadcast] and its helpers. A tag does not observe
// application state or cause an update by itself.
//
// # Choosing tags
//
// Prefer stable identities derived from the authoritative application data being
// rendered. Use a pointer to that data when a widget depends on the object as a
// whole. Use distinct comparable wrapper types for independently changing parts of
// one object. When no data object provides an identity, use a named empty struct for
// a shared signal.
//
// For example, a widget displaying a user's name can listen both to the user and
// specifically to the name:
//
//	type UserData struct {
//		Name string
//	}
//	type userNameTag struct{ User *UserData }
//	type clockTag struct{}
//
//	nameElem.Tag(user, userNameTag{User: user})
//	clockElem.Tag(clockTag{})
//
//	rq.Dirty(userNameTag{User: user}) // the name changed
//	rq.Dirty(user)                    // the user was deleted or changed as a whole
//	rq.Dirty(clockTag{})               // the displayed time changed
//
// Name a tag for the dependency it identifies, not the event that dirties it;
// prefer userNameTag to userDataNameChange.
//
// # Expansion and TagGetter
//
// [TagExpand] recursively flattens tag keys, []Tag and []any collections, and
// [TagGetter] values into unique, usable keys. Keys must be comparable at runtime
// and equal to themselves; see [TagExpand] for the accepted inputs and error behavior.
//
// [TagGetter] controls how a value expands; implementing it does not register an
// Element. [TagGetter.JawsGetTag] may return a nil interface during a documented
// initialization phase. A nil result expands to no keys, and later initialization
// does not affect earlier registration, dirtying, or broadcasts. After the first
// non-nil result, every call must expand to the same key set. Expansion receives no
// Request or rendering context, has no call-count guarantee, and may occur before
// rendering. See [TagGetter] for requirements on returned values and concurrency.
//
// [github.com/linkdata/jaws.Request.Tag] and the normal targeting APIs expand inputs
// in the same way. Returning a shared group key from [TagGetter.JawsGetTag] therefore
// causes dirtying that value to target the group too. Register group dependencies
// separately when item-level dirtying must remain narrow.
//
// # Registration and use
//
// [github.com/linkdata/jaws.Request.Tag] and
// [github.com/linkdata/jaws.Element.Tag] expand and register tags. Registration is
// additive: tags may be added during rendering or updating and remain active until
// the Element is removed or its Request ends. Prefer registering known dependencies
// during initial rendering. Add one during an update only when it is discovered later
// and remains valid for the Element's lifetime.
//
// Adding a tag does not schedule an update or change an operation whose targets were
// already selected. Register a required dependency before calling Dirty or Broadcast;
// do not rely on a later registration observing an earlier operation.
//
// JaWS does not remove individual tag associations. Design each registered key to
// remain a valid dependency for the Element's lifetime. Removing the Element removes
// all of its associations.
//
// [github.com/linkdata/jaws.Request.TagsOf] returns an Element's registered keys.
// [github.com/linkdata/jaws.Request.HasTag] is an advanced test for one
// already-expanded key; it does not expand or validate its argument, and invalid
// values may panic. [github.com/linkdata/jaws.Request.GetElements] expands its input.
package tag
