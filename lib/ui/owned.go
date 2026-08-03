package ui

import "github.com/linkdata/jaws"

// elementOwner is implemented by widgets in this package that create and track
// [jaws.Element] values of their own, so a subtree that has left the browser DOM can
// be unregistered recursively.
//
// Ownership is one level deep per widget: a nested widget tracks the Elements it
// creates itself, and the walk below reaches them through it.
type elementOwner interface {
	// takeOwnedElements returns the tracked Elements and clears the tracking state,
	// so the caller becomes responsible for unregistering them.
	takeOwnedElements() []*jaws.Element
}

// appendOwnedElements appends elems and, recursively, the Elements they own to dst.
//
// Taking each owner's set as the walk proceeds detaches the subtree and makes the
// recursion self-limiting, so an unexpected ownership cycle terminates instead of
// recursing forever. It must not be called with a widget lock held: it locks each
// visited owner in turn.
func appendOwnedElements(dst []*jaws.Element, elems []*jaws.Element) []*jaws.Element {
	for _, elem := range elems {
		dst = append(dst, elem)
		dst = appendOwnedBy(dst, elem)
	}
	return dst
}

// appendOwnedBy appends the Elements owned by elem, recursively, to dst. It does not
// append elem itself, so a caller that unregisters elem separately (through
// [jaws.Element.Remove], say) can still collect its descendants.
func appendOwnedBy(dst []*jaws.Element, elem *jaws.Element) []*jaws.Element {
	if owner, ok := elem.UI().(elementOwner); ok {
		dst = appendOwnedElements(dst, owner.takeOwnedElements())
	}
	return dst
}

// deleteOwnedElements unregisters elems and, recursively, the Elements they own,
// using a single pass over the [jaws.Request] registry.
//
// It queues no browser operations, so callers use it when the DOM holding elems is
// already gone (replaced by [jaws.Element.SetInner], or removed) or was never
// delivered to the browser at all.
func deleteOwnedElements(rq *jaws.Request, elems []*jaws.Element) {
	if len(elems) > 0 {
		// Sized for the common case of elems owning nothing further, so a wide
		// generation does not regrow the slice on the way out.
		rq.DeleteElements(appendOwnedElements(make([]*jaws.Element, 0, len(elems)), elems))
	}
}
