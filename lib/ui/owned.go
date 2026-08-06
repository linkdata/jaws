package ui

import "github.com/linkdata/jaws"

// appendOwnedElements appends elems and, recursively, the Elements they own to dst.
//
// Taking each state's set as the walk proceeds detaches the subtree and makes the
// recursion self-limiting, so an unexpected ownership cycle terminates instead of
// recursing forever. It must not be called with a widget-state lock held: it locks
// each visited state in turn.
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
//
// Ownership is stored in the Element state slot. Match the two private state types
// exactly and check typed nils before calling their methods: the slot may legitimately
// contain any value a renderer claimed. Each take detaches the direct children under
// that state's mutex, and recursion starts only after the mutex is released.
func appendOwnedBy(dst []*jaws.Element, elem *jaws.Element) []*jaws.Element {
	var owned []*jaws.Element
	switch st := jaws.ElementState(elem).(type) {
	case *containerState:
		if st != nil {
			owned = st.takeOwnedElements()
		}
	case *templateState:
		if st != nil {
			owned = st.takeOwnedElements()
		}
	}
	return appendOwnedElements(dst, owned)
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
