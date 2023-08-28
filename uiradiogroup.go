package jaws

import (
	"html/template"
	"strings"
)

type RadioElement struct {
	e *Element
}

func (rq *Request) RadioGroup(params ...interface{}) (rl []RadioElement) {
	up := NewParams(params)
	tags := up.Tags()
	up.attrs = append(up.attrs, `name="`+MakeID()+`"`)
	up.nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			up.nb = nb
			rl = append(rl, RadioElement{e: rq.NewElement(tags, NewUiRadio(up), params)})
		}
	})
	return
}

// Radio renders a HTML input element of type 'radio'.
func (r RadioElement) Radio(attrs ...string) template.HTML {
	var sb strings.Builder
	if err := r.e.ui.(*UiRadio).JawsRender(r.e, &sb); err != nil {
		r.e.rq.Jaws.MustLog(err)
	}
	return template.HTML(sb.String())
}

// Label renders a HTML label element.
func (r *RadioElement) Label(attrs ...string) template.HTML {
	return HtmlInner(0, "label", "", r.e.ui.(*UiRadio).Html(), append(attrs, `for="jid.`+r.e.Jid().String()+`"`)...)
}
