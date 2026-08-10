package ui

import (
	"html/template"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/htmlio"
)

// containerState is the mutable reconciliation state for one container-family Element.
type containerState struct {
	mu sync.Mutex
	// rendering is true from the successful state claim until JawsRender finishes.
	// It keeps an update from using a published but incomplete state.
	rendering bool
	dirtyTag  any
	contents  []*jaws.Element
}

// finishRender publishes the render-time dirty tag, ends the rendering phase, and
// commits contents only when the complete render succeeded.
func (st *containerState) finishRender(dirtyTag any, contents []*jaws.Element, succeeded bool) {
	st.mu.Lock()
	st.dirtyTag = dirtyTag
	if succeeded {
		st.contents = contents
	}
	st.rendering = false
	st.mu.Unlock()
}

// takeOwnedElements returns the child Elements and clears the reconciliation state,
// transferring responsibility for unregistering them to the caller.
func (st *containerState) takeOwnedElements() (owned []*jaws.Element) {
	st.mu.Lock()
	owned, st.contents = st.contents, nil
	st.mu.Unlock()
	return
}

func (st *containerState) deleteContent(elem *jaws.Element) {
	st.deleteContents([]*jaws.Element{elem})
}

func (st *containerState) deleteContents(elems []*jaws.Element) {
	st.mu.Lock()
	st.contents = slices.DeleteFunc(st.contents, func(childElem *jaws.Element) bool {
		return slices.Contains(elems, childElem)
	})
	st.mu.Unlock()
}

// claimContainerState claims elem's state slot with a fresh containerState.
// It returns a nil state when another claimant already occupies the slot.
func claimContainerState(elem *jaws.Element) (st *containerState, err error) {
	st = &containerState{}
	if err = jaws.SetElementState(elem, st); err != nil {
		st = nil
	}
	return
}

// stateForContainerUpdate returns usable container state for elem, claiming an empty
// state when the slot is unclaimed.
func stateForContainerUpdate(elem *jaws.Element) (st *containerState, err error) {
	// A missing state uses the lazy-claim path. Treat an occupied foreign slot,
	// typed nil, in-progress render, or lost concurrent claim as contention.
	switch state := jaws.ElementState(elem).(type) {
	case nil:
		st, err = claimContainerState(elem)
	case *containerState:
		if state == nil {
			err = jaws.ErrElementStateClaimed
		} else {
			state.mu.Lock()
			rendering := state.rendering
			state.mu.Unlock()
			if rendering {
				err = jaws.ErrElementStateClaimed
			} else {
				st = state
			}
		}
	default:
		err = jaws.ErrElementStateClaimed
	}
	return
}

// containerDirtyTag returns elem's completed render-time dirty tag. It never
// claims state or reports contention; absent, foreign, typed-nil and rendering
// states all have no usable dirty tag.
func containerDirtyTag(elem *jaws.Element) (dirtyTag any) {
	if st, ok := jaws.ElementState(elem).(*containerState); ok && st != nil {
		st.mu.Lock()
		if !st.rendering {
			dirtyTag = st.dirtyTag
		}
		st.mu.Unlock()
	}
	return
}

// render renders the Container around its current children. If complete is non-nil,
// it runs after the complete wrapper is written but before the rendering phase ends.
func (u Container) render(elem *jaws.Element, w io.Writer, params []any, complete func()) (err error) {
	st := &containerState{rendering: true}
	if err = jaws.SetElementState(elem, st); err != nil {
		return
	}

	var dirtyTag any
	var contents []*jaws.Element
	succeeded := false
	defer func() {
		st.finishRender(dirtyTag, contents, succeeded)
	}()
	defer func() {
		if !succeeded {
			// Keep the rendering phase active until rollback is complete, so a
			// concurrent update cannot reconcile against a failed partial render.
			deleteOwnedElements(elem.Request, contents)
		}
	}()

	dirtyTag = elem.ApplyGetter(u.children)
	getterAttrs := elem.ApplyInitialHTMLAttr(u.children)
	attrs := append(elem.ApplyParams(params), getterAttrs...)
	b := elem.Jid().AppendStartTagAttr(nil, u.outerHTMLTag)
	b = htmlio.AppendAttrs(b, attrs)
	b = append(b, '>')
	_, err = w.Write(b)
	if err == nil {
		// Validate every child before creating any Element: an unusable child (one
		// that is not comparable at runtime, or not equal to itself) terminates the
		// Request, and the rest must not be rendered. Keeping unusable children out of
		// st.contents also stops a later reconcile pool build from hashing one.
		childUIs := u.children.JawsContains(elem)
		if !cancelUnusableChildren(elem, childUIs) {
			for _, childUI := range childUIs {
				childElem := elem.Request.NewElement(childUI)
				contents = append(contents, childElem)
				if err = childElem.JawsRender(w, nil); err != nil {
					break
				}
			}
		}

		// Always emit the closing tag, even on a child-render error, to balance
		// the start tag already written above. Preserve the original error.
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, u.outerHTMLTag...)
		b = append(b, '>')
		if _, closeErr := w.Write(b); err == nil {
			err = closeErr
		}
	}
	if err == nil && complete != nil {
		complete()
	}

	succeeded = err == nil
	return
}

// update reconciles elem's children and reports whether its state was acquired.
// Ordinary reconciliation, including cancellation for an unusable child or a logged
// append-render error, reports true. State contention reports false and performs no
// application callback or browser work.
func (u Container) update(elem *jaws.Element) (updated bool) {
	st, err := stateForContainerUpdate(elem)
	if err != nil {
		// State helpers release both the Request lock and state mutex before returning;
		// logging invokes application code and must stay outside them.
		elem.Request.MustLog(err)
		return
	}

	wantContents := u.children.JawsContains(elem)
	toAppend, toRemove, alreadyDeleted, oldOrder, newOrder := st.reconcile(elem, wantContents)

	// A deleted old child is already absent from the registry, but anything it
	// owned may still be registered. Live leftovers are removed from the browser
	// first; only a successful removal transfers their descendants to this cleanup.
	var owned []*jaws.Element
	for _, childElem := range alreadyDeleted {
		owned = appendOwnedBy(owned, childElem)
	}
	for _, childElem := range toRemove {
		elem.Remove(childElem)
		if childElem.Deleted() {
			owned = appendOwnedBy(owned, childElem)
		}
	}
	elem.Request.DeleteElements(owned)

	// Reconcile pre-creates every new Element. Track the current and unvisited tail so
	// a child-render or MustLog panic cannot leave never-appended Elements committed in
	// state. Successfully appended children fall off the front and remain committed.
	pendingAppend := toAppend
	if len(pendingAppend) > 0 {
		defer func() {
			if len(pendingAppend) > 0 {
				if panicValue := recover(); panicValue != nil {
					st.deleteContents(pendingAppend)
					deleteOwnedElements(elem.Request, pendingAppend)
					panic(panicValue)
				}
			}
		}()
	}
	for _, childElem := range toAppend {
		var sb strings.Builder
		if renderErr := childElem.JawsRender(&sb, nil); renderErr != nil {
			// Unregister the child and its nested UI before MustLog, which panics when
			// no logger is configured; deferring it would strand the subtree.
			deleteOwnedElements(elem.Request, []*jaws.Element{childElem})
			st.deleteContent(childElem)
			newOrder = slices.DeleteFunc(newOrder, func(id jaws.Jid) bool { return id == childElem.Jid() })
			pendingAppend = pendingAppend[1:]
			elem.Jaws.MustLog(renderErr)
			continue
		}
		elem.Append(template.HTML(sb.String())) // #nosec G203
		pendingAppend = pendingAppend[1:]
	}

	if !slices.Equal(oldOrder, newOrder) {
		elem.Order(newOrder)
	}
	updated = true
	return
}

// cancelUnusableChildren terminates the Request and reports true if any child cannot
// be used as a container pool key: nil, not comparable at runtime, or not equal to
// itself (a value holding NaN). It aborts on the first such child.
func cancelUnusableChildren(elem *jaws.Element, children []jaws.UI) bool {
	// Call without holding a containerState mutex: Request.Cancel runs the user
	// logger synchronously. Validating the whole slice up front also prevents later
	// children from being created once the Request is terminating. The cause matches
	// tag.ErrNotUsableAsTag through jaws.NewErrUnusableUI.
	if bad, ok := firstUnusableChild(children); ok {
		elem.Request.Cancel(jaws.NewErrUnusableUI(bad))
		return true
	}
	return false
}

// firstUnusableChild returns the first child that is nil, not equal to itself, or not
// comparable at runtime, and whether one was found.
func firstUnusableChild(children []jaws.UI) (bad jaws.UI, found bool) {
	// One deferred recover guards the scan, avoiding a defer per child. A comparison
	// panic is attributed to the child currently held in bad.
	defer func() {
		if recover() != nil {
			found = true // comparing bad panicked: not comparable at runtime
		}
	}()
	for _, childUI := range children {
		bad = childUI
		if childUI == nil || childUI != childUI {
			return childUI, true
		}
	}
	return nil, false
}

// reconcile matches st.contents to wantContents under st.mu and returns the Elements
// to append, the live leftovers to remove, already-deleted old Elements whose owned
// descendants need cleanup, and the old and new Jid orders.
func (st *containerState) reconcile(elem *jaws.Element, wantContents []jaws.UI) (toAppend, toRemove, alreadyDeleted []*jaws.Element, oldOrder, newOrder []jaws.Jid) {
	// Validate before locking because cancellation runs the user logger.
	if cancelUnusableChildren(elem, wantContents) {
		return
	}

	// NewElement is the sole deliberate state-mutex-to-Request-lock edge.
	// Application callbacks and all other Request work stay outside st.mu.
	st.mu.Lock()
	defer st.mu.Unlock()

	// Build a pool of reusable Elements keyed by UI, preserving duplicates.
	pool := make(map[jaws.UI][]*jaws.Element, len(st.contents))
	oldOrder = make([]jaws.Jid, len(st.contents))
	for i, childElem := range st.contents {
		oldOrder[i] = childElem.Jid()
		pool[childElem.UI()] = append(pool[childElem.UI()], childElem)
	}

	// Build the new contents, reusing pooled Elements where possible.
	newOrder = make([]jaws.Jid, 0, len(wantContents))
	oldLen := len(st.contents)
	st.contents = st.contents[:0]
	for _, childUI := range wantContents {
		var childElem *jaws.Element
		// Discard and report deleted matches until a live candidate is found. Their
		// nested owned Elements still need recursive registry cleanup after unlocking.
		for elems := pool[childUI]; len(elems) > 0 && childElem == nil; elems = pool[childUI] {
			candidate := elems[0]
			pool[childUI] = elems[1:]
			if candidate.Deleted() {
				alreadyDeleted = append(alreadyDeleted, candidate)
			} else {
				childElem = candidate
			}
		}
		if childElem == nil {
			childElem = elem.Request.NewElement(childUI)
			toAppend = append(toAppend, childElem)
		}
		st.contents = append(st.contents, childElem)
		newOrder = append(newOrder, childElem.Jid())
	}
	// Do not retain removed Elements, or the subtrees reachable through their
	// state, in the vacated tail of the contents backing array.
	if len(st.contents) < oldLen {
		clear(st.contents[len(st.contents):oldLen])
	}

	for _, elems := range pool {
		for _, childElem := range elems {
			if childElem.Deleted() {
				alreadyDeleted = append(alreadyDeleted, childElem)
			} else {
				toRemove = append(toRemove, childElem)
			}
		}
	}
	return
}
