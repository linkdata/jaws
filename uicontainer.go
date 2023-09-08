package jaws

import (
	"bytes"
	"html/template"
	"io"
)

type UiContainer struct {
	OuterHTMLTag string
	Templater
	UiHtml
	state []Template
}

func (ui *UiContainer) JawsTags(rq *Request, tags []interface{}) []interface{} {
	return append(tags, ui.Templater)
}

func (ui *UiContainer) JawsRender(e *Element, w io.Writer) (err error) {
	writeUiDebug(e, w)
	b := e.jid.AppendStartTagAttr(nil, ui.OuterHTMLTag)
	b = e.AppendAttrs(b)
	b = append(b, '>')
	if _, err = w.Write(b); err == nil {
		ui.state = ui.Templater.JawsTemplates(e.Request, nil)
		for _, t := range ui.state {
			elem := e.Request.NewElement(t, nil)
			if err = e.Jaws.Log(t.JawsRender(elem, w)); err != nil {
				break
			}
		}
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, []byte(ui.OuterHTMLTag)...)
		b = append(b, '>')
		_, _ = w.Write(b)
	}
	return
}

func (ui *UiContainer) JawsUpdate(e *Element) (err error) {
	newState := ui.Templater.JawsTemplates(e.Request, nil)
	newMap := make(map[interface{}]struct{})
	for _, t := range newState {
		newMap[t] = struct{}{}
	}

	oldMap := make(map[interface{}]struct{})
	for _, t := range ui.state {
		oldMap[t] = struct{}{}
		if _, ok := newMap[t]; !ok {
			e.Jaws.Remove(t)
		}
	}

	var orderTags []interface{}
	for _, t := range newState {
		orderTags = append(orderTags, t.Dot)
		if _, ok := oldMap[t]; !ok {
			var b bytes.Buffer
			elem := e.Request.NewElement(t, nil)
			if err = e.Jaws.Log(t.JawsRender(elem, &b)); err == nil {
				e.Jaws.Append(ui.Templater, template.HTML(b.String()))
			}
		}
	}

	e.Jaws.Order(orderTags)
	ui.state = newState
	return
}

func NewUiContainer(outerTag string, templater Templater, up Params) *UiContainer {
	return &UiContainer{
		OuterHTMLTag: outerTag,
		Templater:    templater,
		UiHtml:       NewUiHtml(up),
	}
}

func (rq *Request) Container(outerTag string, templater Templater, params ...interface{}) template.HTML {
	return rq.UI(NewUiContainer(outerTag, templater, NewParams(nil, params)), params...)
}
