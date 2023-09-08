package jaws

import (
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
	var b []byte
	b = e.jid.AppendStartTagAttr(b, ui.OuterHTMLTag)
	b = e.AppendAttrs(b)
	b = append(b, '>')
	if _, err = w.Write(b); err == nil {
		ui.state = ui.Templater.JawsTemplates(e.Request, nil)
		for _, item := range ui.state {
			_ = item
		}
		b = b[:0]
		b = append(b, "</"...)
		b = append(b, []byte(ui.OuterHTMLTag)...)
		b = append(b, '>')
		_, _ = w.Write(b)
	}

	return nil
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
