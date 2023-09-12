package jaws

import (
	"bytes"
	"html/template"
	"io"
)

type UiTbody struct {
	UiHtml
	Tagger      Tagger
	RowTemplate *template.Template
	state       []interface{}
}

func (ui *UiTbody) JawsTags(rq *Request, tags []interface{}) []interface{} {
	return append(tags, ui.Tagger)
}

func (ui *UiTbody) JawsRender(e *Element, w io.Writer) {
	writeUiDebug(e, w)
	var b []byte
	b = e.jid.AppendStartTagAttr(b, "tbody")
	b = e.AppendAttrs(b)
	b = append(b, '>')
	_, err := w.Write(b)
	if err == nil {
		ui.state = ui.Tagger.JawsTags(e.Request, nil)
		for _, tag := range ui.state {
			trui := NewUiTemplate(Template{ui.RowTemplate, tag})
			elem := e.Request.NewElement(trui, []interface{}{tag})
			trui.JawsRender(elem, w)
		}
		_, err = w.Write([]byte("</tbody>"))
	}
	maybePanic(err)
}

func (ui *UiTbody) JawsUpdate(u Updater) {
	newState := ui.Tagger.JawsTags(u.Request, nil)
	newMap := make(map[interface{}]struct{})
	for _, tag := range newState {
		newMap[tag] = struct{}{}
	}

	oldMap := make(map[interface{}]struct{})
	for _, tag := range ui.state {
		oldMap[tag] = struct{}{}
		if _, ok := newMap[tag]; !ok {
			u.Jaws.Remove(tag)
		}
	}

	for _, tag := range newState {
		if _, ok := oldMap[tag]; !ok {
			trui := NewUiTr(NewParams(u.Request.Template(ui.RowTemplate, tag), nil))
			elem := u.Request.NewElement(trui, []interface{}{tag})
			var b bytes.Buffer
			trui.JawsRender(elem, &b)
			u.Jaws.Append(ui.Tagger, template.HTML(b.String()))
		}
	}

	u.Jaws.Order(newState)
	ui.state = newState
}

func NewUiTbody(tagger Tagger, rowTemplate *template.Template, up Params) *UiTbody {
	return &UiTbody{
		UiHtml:      NewUiHtml(up),
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
