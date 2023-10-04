package jaws

import (
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/deadlock"
)

type uiWrapContainer struct {
	UiHtml
	Container
	mu       deadlock.Mutex
	contents []*Element
}

func (ui *uiWrapContainer) renderContainer(e *Element, w io.Writer, outerhtmltag string, params []interface{}) {
	ui.parseGetter(e, ui.Container)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	b := e.jid.AppendStartTagAttr(nil, outerhtmltag)
	for _, attr := range attrs {
		b = append(b, ' ')
		b = append(b, attr...)
	}
	b = append(b, '>')
	_, err := w.Write(b)
	if err == nil {
		for _, cui := range ui.Container.JawsContains(e.Request) {
			if elem := e.Request.NewElement(cui); elem != nil {
				ui.contents = append(ui.contents, elem)
				elem.Render(w, nil)
			}
		}
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, outerhtmltag...)
		b = append(b, '>')
		_, err = w.Write(b)
	}
	maybePanic(err)
}

func sameOrder(a, b []Jid) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (ui *uiWrapContainer) JawsUpdate(e *Element) {
	var toRemove, toAppend []*Element
	var orderData []Jid

	oldMap := make(map[UI]*Element)
	newMap := make(map[UI]struct{})
	newContents := ui.Container.JawsContains(e.Request)
	for _, t := range newContents {
		newMap[t] = struct{}{}
	}

	ui.mu.Lock()
	oldOrder := make([]Jid, len(ui.contents))
	for i, elem := range ui.contents {
		oldOrder[i] = elem.jid
		oldMap[elem.ui] = elem
		if _, ok := newMap[elem.ui]; !ok {
			toRemove = append(toRemove, elem)
		}
	}
	ui.contents = ui.contents[:0]
	for _, cui := range newContents {
		var elem *Element
		if elem = oldMap[cui]; elem == nil {
			if elem = e.Request.NewElement(cui); elem == nil {
				continue
			}
			toAppend = append(toAppend, elem)
		}
		ui.contents = append(ui.contents, elem)
		orderData = append(orderData, elem.jid)
	}
	ui.mu.Unlock()

	for _, elem := range toRemove {
		e.Remove(elem.jid.String())
		e.deleteElement(elem)
	}

	for _, elem := range toAppend {
		var sb strings.Builder
		elem.ui.JawsRender(elem, &sb, []any{"hidden"})
		e.Append(template.HTML(sb.String()))
	}

	if !sameOrder(oldOrder, orderData) {
		e.Order(orderData)
	}

	for _, elem := range toAppend {
		elem.Show()
	}
}
