package ui

import (
	"html/template"
	"io"
	"slices"
	"strings"
	"sync"

	pkg "github.com/linkdata/jaws/jaws"
)

// WrapContainer is a helper for widgets that render dynamic child collections.
//
// It tracks previously rendered child elements and performs append/remove/order
// updates during JawsUpdate.
type WrapContainer struct {
	Container pkg.Container
	Tag       any
	mu        sync.Mutex
	contents  []*pkg.Element
}

func NewWrapContainer(c pkg.Container) WrapContainer {
	return WrapContainer{Container: c}
}

func (ui *WrapContainer) RenderContainer(e *pkg.Element, w io.Writer, outerHTMLTag string, params []any) (err error) {
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
			var contents []*pkg.Element
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

func (ui *WrapContainer) UpdateContainer(e *pkg.Element) {
	var toRemove, toAppend []*pkg.Element
	var orderData []pkg.Jid

	oldMap := make(map[pkg.UI]*pkg.Element)
	newMap := make(map[pkg.UI]struct{})
	newContents := ui.Container.JawsContains(e)
	for _, childUI := range newContents {
		newMap[childUI] = struct{}{}
	}

	ui.mu.Lock()
	oldOrder := make([]pkg.Jid, len(ui.contents))
	for i, elem := range ui.contents {
		oldOrder[i] = elem.Jid()
		oldMap[elem.Ui()] = elem
		if _, ok := newMap[elem.Ui()]; !ok {
			toRemove = append(toRemove, elem)
		}
	}
	ui.contents = ui.contents[:0]
	for _, childUI := range newContents {
		elem := oldMap[childUI]
		if elem == nil {
			elem = e.Request.NewElement(childUI)
			toAppend = append(toAppend, elem)
		}
		ui.contents = append(ui.contents, elem)
		orderData = append(orderData, elem.Jid())
	}
	ui.mu.Unlock()

	for _, elem := range toRemove {
		e.Remove(elem.Jid().String())
		e.Request.DeleteElement(elem)
	}

	for _, elem := range toAppend {
		var sb strings.Builder
		must(elem.JawsRender(&sb, nil))
		e.Append(template.HTML(sb.String())) // #nosec G203
	}

	if !slices.Equal(oldOrder, orderData) {
		e.Order(orderData)
	}
}
