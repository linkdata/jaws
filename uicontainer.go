package jaws

import (
	"html/template"
	"io"
	"strings"

	"github.com/linkdata/deadlock"
)

type UiContainer struct {
	OuterHTMLTag string
	UiHtml
	Container
	mu       deadlock.Mutex
	contents []*Element
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.Container)
	attrs := ui.parseParams(e, params)
	writeUiDebug(e, w)
	b := e.jid.AppendStartTagAttr(nil, ui.OuterHTMLTag)
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
				cui.JawsRender(elem, w, nil)
			}
		}
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, []byte(ui.OuterHTMLTag)...)
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

func (ui *UiContainer) JawsUpdate(e *Element) {
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
		elem.Remove()
	}

	for _, elem := range toAppend {
		var sb strings.Builder
		elem.ui.JawsRender(elem, &sb, nil)
		e.Append(template.HTML(sb.String()))
	}

	if !sameOrder(oldOrder, orderData) {
		e.Order(orderData)
	}
}

func NewUiContainer(outerTag string, cont Container) *UiContainer {
	return &UiContainer{
		OuterHTMLTag: outerTag,
		Container:    cont,
	}
}

func (rq *Request) Container(outerTag string, cont Container, params ...interface{}) template.HTML {
	return rq.UI(NewUiContainer(outerTag, cont), params...)
}
