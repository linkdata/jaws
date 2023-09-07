package jaws

import (
	"bytes"
	"html/template"
	"io"
)

type UiTbody struct {
	Tagger      Tagger
	RowTemplate *template.Template
	state       []interface{}
}

func (ui *UiTbody) JawsTags(rq *Request, tags []interface{}) []interface{} {
	return append(tags, ui.Tagger)
}

func (ui *UiTbody) JawsRender(e *Element, w io.Writer) (err error) {
	var b []byte
	b = append(b, "<tbody"...)
	b = e.jid.AppendAttr(b)
	b = e.AppendAttrs(b)
	b = append(b, '>')
	if _, err = w.Write(b); err == nil {
		ui.state = ui.Tagger.JawsTags(e.Request, nil)
		for _, tag := range ui.state {
			trui := NewUiTr(NewParams(e.Request.Template(ui.RowTemplate, tag), nil))
			elem := e.Request.NewElement(trui, []interface{}{tag})
			if err = e.Jaws.Log(trui.JawsRender(elem, w)); err != nil {
				break
			}
		}
		_, _ = w.Write([]byte("</tbody>"))
	}
	return
}

func (ui *UiTbody) JawsUpdate(e *Element) (err error) {
	newState := ui.Tagger.JawsTags(e.Request, nil)
	newMap := make(map[interface{}]struct{})
	for _, tag := range newState {
		newMap[tag] = struct{}{}
	}

	oldMap := make(map[interface{}]struct{})
	for _, tag := range ui.state {
		oldMap[tag] = struct{}{}
		if _, ok := newMap[tag]; !ok {
			e.Jaws.Remove(tag)
		}
	}

	for _, tag := range newState {
		if _, ok := oldMap[tag]; !ok {
			trui := NewUiTr(NewParams(e.Request.Template(ui.RowTemplate, tag), nil))
			elem := e.Request.NewElement(trui, []interface{}{tag})
			var b bytes.Buffer
			if err = e.Jaws.Log(trui.JawsRender(elem, &b)); err == nil {
				e.Jaws.Append(ui.Tagger, template.HTML(b.String()))
			}
		}
	}

	e.Jaws.Order(newState)
	ui.state = newState
	return
}

func NewUiTbody(tagger Tagger, rowTemplate *template.Template, up Params) *UiTbody {
	return &UiTbody{
		Tagger:      tagger,
		RowTemplate: rowTemplate,
	}
}

func (rq *Request) Tbody(tagger Tagger, rowTemplate interface{}, params ...interface{}) template.HTML {
	var templ *template.Template
	if name, ok := rowTemplate.(string); ok {
		templ = rq.Jaws.Template.Lookup(name)
	} else {
		templ = rowTemplate.(*template.Template)
	}
	return rq.UI(NewUiTbody(tagger, templ, NewParams(nil, params)), params...)
}
