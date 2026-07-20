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

// ContainerHelper is a helper for widgets that render dynamic child collections.
//
// It tracks already-rendered child elements and performs append/remove/order
// updates during [ContainerHelper.UpdateContainer].
//
// A ContainerHelper retains child-element reconciliation state for one live
// [jaws.Element]. A widget embedding one must therefore back at most one live
// Element.
//
// A ContainerHelper belongs to a request-scoped widget instance (for example a
// widget created via a RequestWriter helper method).
//
// Error model:
// Child render/update failures are treated as application bugs. Initial-render
// errors are returned to the caller, and update-time append render errors are
// reported through MustLog (which may panic when no logger is configured).
// Update-time child reconciliation is intentionally not transactional: queued
// browser updates cannot be made atomic, and a rollback path would add hot-path
// bookkeeping for a user-code render failure without providing a strong
// correctness guarantee. A newly appended child that fails to render is removed
// from the request and omitted from the browser append/order batch, so a later
// update can retry it from fresh state. Treat such failures as application bugs
// and reload or recover at the application level if needed.
type ContainerHelper struct {
	Container jaws.Container
	// tag is the dirty tag, written once during RenderContainer and read on the
	// event goroutine (Select.JawsInput). The render-completes-before-events
	// lifecycle makes the unsynchronized access safe; it is unexported so external
	// code cannot mutate it.
	tag      any
	mu       sync.Mutex
	contents []*jaws.Element
}

// NewContainerHelper returns a ContainerHelper for rendering and updating c.
//
// The returned value is request-scoped and retains state for one live
// [jaws.Element].
func NewContainerHelper(c jaws.Container) ContainerHelper {
	return ContainerHelper{Container: c}
}

// RenderContainer renders outerHTMLTag around the current children from
// [jaws.Container.JawsContains].
func (u *ContainerHelper) RenderContainer(elem *jaws.Element, w io.Writer, outerHTMLTag string, params []any) (err error) {
	var getterAttrs []template.HTMLAttr
	if u.tag, getterAttrs, err = elem.ApplyGetter(u.Container); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		b := elem.Jid().AppendStartTagAttr(nil, outerHTMLTag)
		b = htmlio.AppendAttrs(b, attrs)
		b = append(b, '>')
		_, err = w.Write(b)
		if err == nil {
			var contents []*jaws.Element
			// Validate every child before creating any Element: an unusable child (one
			// that is not comparable at runtime, or not equal to itself) terminates the
			// Request, and the rest must not be rendered. Keeping unusable children out of
			// u.contents also stops a later reconcile pool build from hashing one.
			children := u.Container.JawsContains(elem)
			if !cancelUnusableChildren(elem, children) {
				for _, childUI := range children {
					childElem := elem.Request.NewElement(childUI)
					contents = append(contents, childElem)
					if err = childElem.JawsRender(w, nil); err != nil {
						break
					}
				}
			}
			// Always emit the closing tag, even on a child-render error, to balance
			// the start tag already written above; leaving it unclosed would be
			// worse for any partial output. The original err is preserved (err2 is
			// only adopted when err is nil).
			b = b[:0]
			b = append(b, "</"...)
			b = append(b, outerHTMLTag...)
			b = append(b, '>')
			if _, err2 := w.Write(b); err == nil {
				err = err2
			}
			// Commit the rendered children only on full success. Any failure — a
			// child render or the closing-tag write above — deletes the child
			// Elements created during this render; otherwise they leak in the
			// Request registry, since RequestWriter.NewUI deletes only the parent
			// Element on a failed render.
			if err == nil {
				u.mu.Lock()
				u.contents = contents
				u.mu.Unlock()
			} else {
				for _, childElem := range contents {
					elem.Request.DeleteElement(childElem)
				}
			}
		}
	}
	return
}

// UpdateContainer updates child elements to match [jaws.Container.JawsContains].
//
// Render errors for newly appended children are reported through
// [jaws.Jaws.MustLog], which may panic when no [jaws.Jaws.Logger] is configured.
func (u *ContainerHelper) UpdateContainer(elem *jaws.Element) {
	wantContents := u.Container.JawsContains(elem)
	toAppend, toRemove, oldOrder, newOrder := u.reconcile(elem, wantContents)

	// Remove leftover Elements from both the browser DOM and the Request registry.
	for _, childElem := range toRemove {
		elem.Remove(childElem)
	}

	for _, childElem := range toAppend {
		var sb strings.Builder
		if err := childElem.JawsRender(&sb, nil); err != nil {
			elem.Request.DeleteElement(childElem)
			u.deleteContent(childElem)
			newOrder = slices.DeleteFunc(newOrder, func(id jaws.Jid) bool { return id == childElem.Jid() })
			elem.Jaws.MustLog(err)
			continue
		}
		elem.Append(template.HTML(sb.String())) // #nosec G203
	}

	if !slices.Equal(oldOrder, newOrder) {
		elem.Order(newOrder)
	}
}

func (u *ContainerHelper) deleteContent(elem *jaws.Element) {
	u.mu.Lock()
	u.contents = slices.DeleteFunc(u.contents, func(childElem *jaws.Element) bool { return childElem == elem })
	u.mu.Unlock()
}

// cancelUnusableChildren terminates the Request and reports true if any child cannot
// be used as a container pool key: nil, not comparable at runtime, or not equal to
// itself (a value holding NaN). It aborts on the first such child.
//
// It must be called without holding u.mu. [jaws.Request.Cancel] runs the user logger
// synchronously, and the jaws locking contract forbids that under a lock; a logger
// re-entering the container would otherwise deadlock. Validating the whole slice up
// front also stops the caller creating or rendering later children once the Request
// is terminating. The cancellation cause matches tag.ErrNotUsableAsTag with
// errors.Is (see [jaws.NewErrUnusableUI]).
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

// reconcile matches u.contents to wantContents under u.mu and returns the
// Elements to append, the leftover Elements to remove, and the old and new Jid
// orders.
//
// The wanted children are validated with cancelUnusableChildren before u.mu is
// locked, since that terminates the Request through the user logger, which the
// locking contract forbids under a lock. A non-reflexive value (one holding NaN)
// would miss the pool lookup and a runtime-incomparable one would panic hashing it,
// so on the first such child the Request is terminated and reconcile returns nothing
// to append, remove or reorder. The lock is still released with defer as a further
// guard against a panic leaving u.mu held, mirroring jaws.setDirty.
func (u *ContainerHelper) reconcile(elem *jaws.Element, wantContents []jaws.UI) (toAppend, toRemove []*jaws.Element, oldOrder, newOrder []jaws.Jid) {
	if cancelUnusableChildren(elem, wantContents) {
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	// build pool of reusable Elements keyed by UI, preserving duplicates
	pool := make(map[jaws.UI][]*jaws.Element, len(u.contents))
	oldOrder = make([]jaws.Jid, len(u.contents))
	for i, childElem := range u.contents {
		oldOrder[i] = childElem.Jid()
		pool[childElem.UI()] = append(pool[childElem.UI()], childElem)
	}

	// build new contents, reusing pooled Elements where possible
	newOrder = make([]jaws.Jid, 0, len(wantContents))
	u.contents = u.contents[:0]
	for _, childUI := range wantContents {
		var childElem *jaws.Element
		// Reuse a pooled Element, discarding any that were deleted out-of-band (a
		// what.Delete broadcast targeting a tag the child registered, or a browser
		// what.Remove). A deleted Element is inert, so reusing it would leave the
		// child permanently unrendered and put a phantom Jid in the Order; falling
		// through to NewElement re-creates it so the container self-heals.
		for elems := pool[childUI]; len(elems) > 0 && childElem == nil; elems = pool[childUI] {
			candidate := elems[0]
			pool[childUI] = elems[1:]
			if !candidate.Deleted() {
				childElem = candidate
			}
		}
		if childElem == nil {
			childElem = elem.Request.NewElement(childUI)
			toAppend = append(toAppend, childElem)
		}
		u.contents = append(u.contents, childElem)
		newOrder = append(newOrder, childElem.Jid())
	}

	// A deleted leftover is already unregistered, while Element.Remove requires a
	// live, registered child.
	for _, elems := range pool {
		for _, e := range elems {
			if !e.Deleted() {
				toRemove = append(toRemove, e)
			}
		}
	}
	return
}
