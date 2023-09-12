package jaws

import (
	"bytes"
	"html/template"
	"io"

	"github.com/linkdata/deadlock"
)

type UiContainer struct {
	OuterHTMLTag string
	Container
	UiHtml
	mu    deadlock.Mutex
	state []Template
}

func (ui *UiContainer) JawsTags(rq *Request, tags []interface{}) []interface{} {
	return append(tags, ui.Container)
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer) {
	writeUiDebug(e, w)
	b := e.jid.AppendStartTagAttr(nil, ui.OuterHTMLTag)
	b = e.AppendAttrs(b)
	b = append(b, '>')
	_, err := w.Write(b)
	if err == nil {
		ui.state = ui.Container.JawsContains(e.Request)
		for _, t := range ui.state {
			elem := e.Request.NewElement(t, nil)
			t.JawsRender(elem, w)
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
	var toRemove, toAppend []Template
	var orderTags []interface{}

	newState := ui.Container.JawsContains(u.Request)
	newMap := make(map[Template]struct{})
	for _, t := range newState {
		newMap[t] = struct{}{}
	}
	oldMap := make(map[Template]struct{})

	ui.mu.Lock()
	for _, t := range ui.state {
		oldMap[t] = struct{}{}
		if _, ok := newMap[t]; !ok {
			toRemove = append(toRemove, t)
		}
	}
	for _, t := range newState {
		orderTags = append(orderTags, t.Dot)
		if _, ok := oldMap[t]; !ok {
			toAppend = append(toAppend, t)
		}
	}
	ui.state = newState
	ui.mu.Unlock()

	for _, t := range toRemove {
		u.Jaws.Remove(t)
	}

	for _, t := range toAppend {
		var b bytes.Buffer
		elem := u.Request.NewElement(t, nil)
		t.JawsRender(elem, &b)
		u.Jaws.Append(ui.Container, template.HTML(b.String()))
	}

	u.Jaws.Order(orderTags)
}

func NewUiContainer(outerTag string, cont Container, up Params) *UiContainer {
	return &UiContainer{
		OuterHTMLTag: outerTag,
		Container:    cont,
		UiHtml:       NewUiHtml(up),
	}
}

func (rq *Request) Container(outerTag string, cont Container, params ...interface{}) template.HTML {
	return rq.UI(NewUiContainer(outerTag, cont, NewParams(nil, params)), params...)
}
