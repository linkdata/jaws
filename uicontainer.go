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

func (ui *UiContainer) fillState(rq *Request, state []Template) []Template {
	state = append(state, ui.Container.JawsContains(rq)...)
	for i := range state {
		state[i].Container = ui.Container
	}
	return state
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer) {
	writeUiDebug(e, w)
	b := e.jid.AppendStartTagAttr(nil, ui.OuterHTMLTag)
	b = e.AppendAttrs(b)
	b = append(b, '>')
	_, err := w.Write(b)
	if err == nil {
		ui.state = ui.fillState(e.Request, nil)
		for i := range ui.state {
			elem := e.Request.NewElement(ui.state[i], nil)
			ui.state[i].JawsRender(elem, w)
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
	var toAppend []Template
	var toRemove, orderTags []interface{}

	newState := ui.fillState(u.Request, nil)
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
		orderTags = append(orderTags, t)
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
