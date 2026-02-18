package ui

import (
	"html/template"
	"io"
	"slices"
	"strings"
	"sync"

	"github.com/linkdata/jaws/core"
)

// ContainerHelper is a helper for widgets that render dynamic child collections.
//
// It tracks previously rendered child elements and performs append/remove/order
// updates during JawsUpdate.
//
// A ContainerHelper belongs to a widget instance and is intended for render-scoped
// widget lifetimes (for example widgets created via RequestWriter helper methods).
type ContainerHelper struct {
	Container core.Container
	Tag       any
	mu        sync.Mutex
	contents  []*core.Element
}

func NewContainerHelper(c core.Container) ContainerHelper {
	return ContainerHelper{Container: c}
}

func (ui *ContainerHelper) RenderContainer(e *core.Element, w io.Writer, outerHTMLTag string, params []any) (err error) {
	if ui.Tag, err = e.ApplyGetter(ui.Container); err == nil {
		attrs := e.ApplyParams(params)
		b := e.Jid().AppendStartTagAttr(nil, outerHTMLTag)
		for _, attr := range attrs {
			b = append(b, ' ')
			b = append(b, attr...)
		}
		b = append(b, '>')
		_, err = w.Write(b)
		if err == nil {
			var contents []*core.Element
			for _, childUI := range ui.Container.JawsContains(e) {
				elem := e.Request.NewElement(childUI)
				if err = elem.JawsRender(w, nil); err != nil {
					break
				}
				contents = append(contents, elem)
			}
			ui.mu.Lock()
			ui.contents = contents
			ui.mu.Unlock()
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

func (ui *ContainerHelper) UpdateContainer(e *core.Element) {
	var toAppend []*core.Element

	wantContents := ui.Container.JawsContains(e)
	newOrder := make([]core.Jid, 0, len(wantContents))

	ui.mu.Lock()
	// build pool of reusable Elements keyed by UI, preserving duplicates
	pool := make(map[core.UI][]*core.Element, len(ui.contents))
	oldOrder := make([]core.Jid, len(ui.contents))
	for i, elem := range ui.contents {
		oldOrder[i] = elem.Jid()
		pool[elem.Ui()] = append(pool[elem.Ui()], elem)
	}

	// build new contents, reusing pooled Elements where possible
	ui.contents = ui.contents[:0]
	for _, childUI := range wantContents {
		var elem *core.Element
		if elems := pool[childUI]; len(elems) > 0 {
			elem = elems[0]
			pool[childUI] = elems[1:]
		} else {
			elem = e.Request.NewElement(childUI)
			toAppend = append(toAppend, elem)
		}
		ui.contents = append(ui.contents, elem)
		newOrder = append(newOrder, elem.Jid())
	}
	ui.mu.Unlock()

	// remove leftover Elements not present in new contents
	for _, elems := range pool {
		for _, elem := range elems {
			e.Remove(elem.Jid().String())
			e.Request.DeleteElement(elem)
		}
	}

	for _, elem := range toAppend {
		var sb strings.Builder
		e.Jaws.MustLog(elem.JawsRender(&sb, nil))
		e.Append(template.HTML(sb.String())) // #nosec G203
	}

	if !slices.Equal(oldOrder, newOrder) {
		e.Order(newOrder)
	}
}
