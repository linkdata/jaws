package jaws

import (
	"bytes"
	"html/template"
	"io"

	"github.com/linkdata/deadlock"
	"github.com/linkdata/jaws/what"
)

type UiContainer struct {
	OuterHTMLTag string
	Container
	UiHtml
	mu       deadlock.Mutex
	contents []*Element
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer, params []interface{}) {
	attrs := ui.parseParams(e, append(params, ui.Container))
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

func (ui *UiContainer) JawsUpdate(u Updater) {
	var toRemove, toAppend []*Element
	var orderData []byte

	oldMap := make(map[UI]*Element)
	newMap := make(map[UI]struct{})
	newContents := ui.Container.JawsContains(u.Request)
	for _, t := range newContents {
		newMap[t] = struct{}{}
	}

	ui.mu.Lock()
	for _, elem := range ui.contents {
		oldMap[elem.ui] = elem
		if _, ok := newMap[elem.ui]; !ok {
			toRemove = append(toRemove, elem)
		}
	}
	ui.contents = ui.contents[:0]
	for i, cui := range newContents {
		var elem *Element
		if elem = oldMap[cui]; elem == nil {
			if elem = u.Request.NewElement(cui); elem == nil {
				continue
			}
			toAppend = append(toAppend, elem)
		}
		ui.contents = append(ui.contents, elem)
		if i > 0 {
			orderData = append(orderData, ' ')
		}
		orderData = elem.jid.AppendInt(orderData)
	}
	ui.mu.Unlock()

	for _, elem := range toRemove {
		u.Request.send(u.outCh, wsMsg{
			Jid:  elem.jid,
			What: what.Remove,
		})
	}

	var b bytes.Buffer
	for _, elem := range toAppend {
		b.Reset()
		elem.ui.JawsRender(elem, &b, nil)
		u.Request.send(u.outCh, wsMsg{
			Jid:  u.Jid(),
			What: what.Append,
			Data: b.String(),
		})
	}

	u.Request.send(u.outCh, wsMsg{
		Jid:  u.Jid(),
		What: what.Order,
		Data: string(orderData),
	})
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
