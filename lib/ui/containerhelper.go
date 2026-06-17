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
// A ContainerHelper belongs to a widget instance and is intended for render-scoped
// widget lifetimes (for example widgets created via RequestWriter helper methods).
//
// Error model:
// Child render/update failures are treated as application bugs. Initial-render
// errors are returned to the caller, and update-time append render errors are
// reported through MustLog (which may panic when no logger is configured).
// Update-time child reconciliation is intentionally not transactional: queued
// browser updates cannot be made atomic, and a rollback path would add hot-path
// bookkeeping for a user-code render failure without providing a strong
// correctness guarantee. Treat such failures as application bugs and reload or
// recover at the application level if needed.
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
// ContainerHelper values are render-scoped and should not be reused across requests.
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
			for _, childUI := range u.Container.JawsContains(elem) {
				childElem := elem.Request.NewElement(childUI)
				contents = append(contents, childElem)
				if err = childElem.JawsRender(w, nil); err != nil {
					break
				}
			}
			if err == nil {
				u.mu.Lock()
				u.contents = contents
				u.mu.Unlock()
			} else {
				for _, childElem := range contents {
					elem.Request.DeleteElement(childElem)
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

	// remove leftover Elements not present in new contents
	for _, childElem := range toRemove {
		elem.Remove(childElem.Jid().String())
		elem.Request.DeleteElement(childElem)
	}

	for _, childElem := range toAppend {
		var sb strings.Builder
		elem.Jaws.MustLog(childElem.JawsRender(&sb, nil))
		elem.Append(template.HTML(sb.String())) // #nosec G203
	}

	if !slices.Equal(oldOrder, newOrder) {
		elem.Order(newOrder)
	}
}

// reconcile matches u.contents to wantContents under u.mu and returns the
// Elements to append, the leftover Elements to remove, and the old and new Jid
// orders. The lock is released with defer so that using a child UI as a map key
// cannot leave u.mu held if it panics: a UI that passes the static comparability
// check can still be non-comparable at runtime (a comparable struct holding e.g.
// a func in an interface field). This mirrors the defense in jaws.setDirty.
func (u *ContainerHelper) reconcile(elem *jaws.Element, wantContents []jaws.UI) (toAppend, toRemove []*jaws.Element, oldOrder, newOrder []jaws.Jid) {
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

	for _, elems := range pool {
		toRemove = append(toRemove, elems...)
	}
	return
}
