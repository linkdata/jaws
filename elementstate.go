package jaws

import "errors"

// ErrElementStateClaimed is returned by [SetElementState] when the [Element] already
// has widget state, including state of the same type.
var ErrElementStateClaimed = errors.New("jaws: element state already claimed")

// ErrElementStateNil is returned by [SetElementState] when the state to store is a nil
// interface, which cannot be distinguished from an unclaimed slot.
var ErrElementStateNil = errors.New("jaws: element state must not be nil")

// ElementState returns the widget state stored for elem, or nil if none was claimed.
//
// The state belongs to the [Element], not to the widget value that claimed it, so any
// widget of the claiming type recognizes it. Loading never claims: a widget that finds
// no state did not claim this Element. Most widgets never claim one, so a nil return
// says nothing about whether the Element was rendered.
//
// Safe for concurrent use; it takes the [Request] lock. Only the slot itself is
// synchronized: whatever the stored value contains is guarded by that value's own
// synchronization, not by this call.
//
// elem must belong to a [Request]: ElementState dereferences elem.Request, so a nil
// elem, or one not obtained from [Request.NewElement], panics.
func ElementState(elem *Element) (state any) {
	rq := elem.Request
	rq.mu.RLock()
	state = elem.data
	rq.mu.RUnlock()
	return
}

// SetElementState claims elem's widget state slot, which a widget does while rendering
// the Element so its updates and cleanup can find that state again.
//
// There is one slot per Element and it cannot be replaced, only claimed: a second claim
// returns [ErrElementStateClaimed] and leaves the stored state untouched, even when the
// new state has the same type. At most one widget may claim a given Element, so a widget
// that renders an Element and delegates to another renderer on that same Element must
// decide which of them claims it.
//
// A nil state returns [ErrElementStateNil] and stores nothing, since a nil interface is
// how an unclaimed slot is represented; that check comes first, so a nil state is
// rejected whatever the slot holds, and before elem is examined at all. A typed nil is a
// non-nil interface and does claim the slot.
//
// Safe for concurrent use: concurrent claims on one Element are serialized by the
// [Request] lock and exactly one wins, the rest reporting [ErrElementStateClaimed]. Only
// the claim is synchronized; mutating the stored value afterwards is guarded by that
// value's own synchronization, not by this call.
//
// elem must belong to a [Request]: SetElementState dereferences elem.Request, so a nil
// elem, or one not obtained from [Request.NewElement], panics — except when state is nil,
// which is rejected before elem is touched.
func SetElementState(elem *Element, state any) error {
	// The two functions are package-level rather than methods on Element because
	// ui.With embeds both *Element and ui.RequestWriter, so any method returning a value
	// is reachable from a template: {{$.Element.SetElementState $.Dot}} would let a
	// template claim the slot out from under the renderer. html/template cannot call a
	// package-level function.
	if state == nil {
		return ErrElementStateNil
	}
	rq := elem.Request
	rq.mu.Lock()
	defer rq.mu.Unlock()
	if elem.data != nil {
		return ErrElementStateClaimed
	}
	elem.data = state
	return nil
}
