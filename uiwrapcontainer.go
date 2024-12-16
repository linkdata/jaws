package jaws

import (
	"html/template"
	"io"
	"slices"
	"strings"

	"github.com/linkdata/deadlock"
)

type uiWrapContainer struct {
	Container
	Tag      any
	mu       deadlock.Mutex
	contents []*Element
}

func (ui *uiWrapContainer) renderContainer(e *Element, w io.Writer, outerhtmltag string, params []any) (err error) {
	ui.Tag = e.ApplyGetter(ui.Container)
	attrs := e.ApplyParams(params)
	b := e.Jid().AppendStartTagAttr(nil, outerhtmltag)
	for _, attr := range attrs {
		b = append(b, ' ')
		b = append(b, attr...)
	}
	b = append(b, '>')
	_, err = w.Write(b)
	if err == nil {
		for _, cui := range ui.Container.JawsContains(e) {
			if err == nil {
				elem := e.Request.NewElement(cui)
				ui.contents = append(ui.contents, elem)
				err = elem.JawsRender(w, nil)
			}
		}
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, outerhtmltag...)
		b = append(b, '>')
		if _, err2 := w.Write(b); err == nil {
			err = err2
		}
	}
	return err
}

func (ui *uiWrapContainer) JawsUpdate(e *Element) {
	var toRemove, toAppend []*Element
	var orderData []Jid

	oldMap := make(map[UI]*Element)
	newMap := make(map[UI]struct{})
	newContents := ui.Container.JawsContains(e)
	for _, t := range newContents {
		newMap[t] = struct{}{}
	}

	ui.mu.Lock()
	oldOrder := make([]Jid, len(ui.contents))
	for i, elem := range ui.contents {
		oldOrder[i] = elem.Jid()
		oldMap[elem.Ui()] = elem
		if _, ok := newMap[elem.Ui()]; !ok {
			toRemove = append(toRemove, elem)
		}
	}
	ui.contents = ui.contents[:0]
	for _, cui := range newContents {
		var elem *Element
		if elem = oldMap[cui]; elem == nil {
			elem = e.Request.NewElement(cui)
			toAppend = append(toAppend, elem)
		}
		ui.contents = append(ui.contents, elem)
		orderData = append(orderData, elem.Jid())
	}
	ui.mu.Unlock()

	for _, elem := range toRemove {
		e.Remove(elem.Jid().String())
		e.Request.deleteElement(elem)
	}

	for _, elem := range toAppend {
		var sb strings.Builder
		maybePanic(elem.JawsRender(&sb, nil))
		e.Append(template.HTML(sb.String())) // #nosec G203
	}

	if !slices.Equal(oldOrder, orderData) {
		e.Order(orderData)
	}
}
