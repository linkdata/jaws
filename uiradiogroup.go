package jaws

import (
	"html/template"
	"strings"
)

type RadioElement struct {
	e *Element
}

func (rq *Request) RadioGroup(namedBoolArray interface{}, params ...interface{}) (rl []RadioElement) {
	up := NewParams(namedBoolArray, params)
	up.attrs = append(up.attrs, `name="`+MakeID()+`"`)
	up.nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			up.vp = nb
			rl = append(rl, RadioElement{e: rq.NewElement(NewUiRadio(up), params)})
		}
	})
	return
}

// Radio renders a HTML input element of type 'radio'.
func (r RadioElement) Radio(attrs ...string) template.HTML {
	var sb strings.Builder
	if err := r.e.ui.(*UiRadio).JawsRender(r.e, &sb); err != nil {
		r.e.Jaws.MustLog(err)
	}
	return template.HTML(sb.String())
}

// Label renders a HTML label element.
func (r *RadioElement) Label(attrs ...string) template.HTML {
	if nb, ok := r.e.ui.(*UiRadio).ValueProxy.(*NamedBool); ok {
		var b []byte
		b = append(b, `for="`...)
		b = r.e.jid.Append(b)
		b = append(b, '"')
		return HtmlInner(0, "label", "", nb.Html(), append(attrs, string(b))...)
	}
	return ""

}
