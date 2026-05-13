package ui

import (
	"html/template"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/linkdata/jaws"
)

// ContainerHelper is a helper for widgets that render dynamic child collections.
//
// It tracks previously rendered child elements and performs append/remove/order
// updates during [ContainerHelper.UpdateContainer].
//
// A ContainerHelper belongs to a widget instance and is intended for render-scoped
// widget lifetimes (for example widgets created via RequestWriter helper methods).
//
// Error model:
// Child render/update failures are treated as application bugs. Initial-render
// errors are returned to the caller, and update-time append render errors are
// reported through MustLog (which may panic when no logger is configured).
// After such failures, DOM and request-tracked element state may be partially
// updated and therefore inconsistent until the next full render/reload.
type ContainerHelper struct {
	Container jaws.Container
	Tag       any
	mu        sync.Mutex
	contents  []*jaws.Element
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
	if u.Tag, getterAttrs, err = elem.ApplyGetter(u.Container); err == nil {
		attrs := append(elem.ApplyParams(params), getterAttrs...)
		b := elem.Jid().AppendStartTagAttr(nil, outerHTMLTag)
		for _, attr := range attrs {
			b = append(b, ' ')
			b = append(b, attr...)
		}
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
func (u *ContainerHelper) UpdateContainer(elem *jaws.Element) {
	var toAppend []*jaws.Element

	wantContents := u.Container.JawsContains(elem)
	newOrder := make([]jaws.Jid, 0, len(wantContents))

	u.mu.Lock()
	// build pool of reusable Elements keyed by UI, preserving duplicates
	pool := make(map[jaws.UI][]*jaws.Element, len(u.contents))
	oldOrder := make([]jaws.Jid, len(u.contents))
	for i, childElem := range u.contents {
		oldOrder[i] = childElem.Jid()
		pool[childElem.Ui()] = append(pool[childElem.Ui()], childElem)
	}

	// build new contents, reusing pooled Elements where possible
	u.contents = u.contents[:0]
	for _, childUI := range wantContents {
		var childElem *jaws.Element
		if elems := pool[childUI]; len(elems) > 0 {
			childElem = elems[0]
			pool[childUI] = elems[1:]
		} else {
			childElem = elem.Request.NewElement(childUI)
			toAppend = append(toAppend, childElem)
		}
		u.contents = append(u.contents, childElem)
		newOrder = append(newOrder, childElem.Jid())
	}
	u.mu.Unlock()

	// remove leftover Elements not present in new contents
	for _, elems := range pool {
		for _, childElem := range elems {
			elem.Remove(childElem.Jid().String())
			elem.Request.DeleteElement(childElem)
		}
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
