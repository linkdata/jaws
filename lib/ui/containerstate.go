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
// The widget value holds only the comparable definition shared across Elements.
type containerState struct {
	mu sync.Mutex
	// rendering is true from the successful state claim until JawsRender finishes.
	// It keeps an update from treating a published but incomplete state as the lazy
	// state of an update-only Register Element.
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

// stateForContainerUpdate returns the state claimed for elem, lazily claiming an
// empty state when the slot is unclaimed. Update-only Register is the intended path,
// but the state slot does not encode how the Element was created. An occupied foreign
// slot, a typed nil state, an in-progress render, or a lost concurrent claim is
// contention.
func stateForContainerUpdate(elem *jaws.Element) (st *containerState, err error) {
	switch state := jaws.ElementState(elem).(type) {
	case nil:
		st = &containerState{}
		if err = jaws.SetElementState(elem, st); err != nil {
			st = nil
		}
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

	var getterAttrs []template.HTMLAttr
	dirtyTag, getterAttrs = elem.ApplyGetter(u.children)
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
//
// It must be called without holding a containerState mutex. [jaws.Request.Cancel]
// runs the user logger synchronously, and the jaws locking contract forbids that under
// a lock. Validating the whole slice up front also stops the caller creating or
// rendering later children once the Request is terminating. The cancellation cause
// matches tag.ErrNotUsableAsTag with errors.Is (see [jaws.NewErrUnusableUI]).
func cancelUnusableChildren(elem *jaws.Element, children []jaws.UI) bool {
	if bad, ok := firstUnusableChild(children); ok {
		elem.Request.Cancel(jaws.NewErrUnusableUI(bad))
		return true
	}
	return false
}

// firstUnusableChild returns the first child that is nil, not equal to itself, or not
// comparable at runtime, and whether one was found.
//
// A single deferred recover guards the whole scan, so a usable child costs only one
// self-comparison rather than a per-child deferred check: comparing a
// runtime-incomparable value panics, which the recover attributes to the child being
// examined (bad). This keeps the common all-usable case cheap on the container update
// hot path.
func firstUnusableChild(children []jaws.UI) (bad jaws.UI, found bool) {
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
//
// The wanted children are validated before st.mu is locked, since cancellation runs
// the user logger. The lock is still released with defer as a guard against a panic.
// Reconciliation's call to [jaws.Request.NewElement] is the sole deliberate
// state-mutex-to-Request-lock edge; application callbacks and all other Request work
// stay outside st.mu.
func (st *containerState) reconcile(elem *jaws.Element, wantContents []jaws.UI) (toAppend, toRemove, alreadyDeleted []*jaws.Element, oldOrder, newOrder []jaws.Jid) {
	if cancelUnusableChildren(elem, wantContents) {
		return
	}

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
