package ui

import "github.com/linkdata/jaws"

// appendOwnedElements appends elems and, recursively, the Elements they own to dst.
func appendOwnedElements(dst []*jaws.Element, elems []*jaws.Element) []*jaws.Element {
	// Taking each state's set detaches the subtree, so an unexpected ownership
	// cycle terminates. Call without a widget-state lock held: this walk locks each
	// visited state in turn.
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
	// Match exact state types and check typed nils: any renderer may claim the slot.
	// Each take detaches direct children before recursion starts.
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
