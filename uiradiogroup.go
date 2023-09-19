package jaws

import (
	"html/template"
	"strings"
)

type RadioElement struct {
	e        *Element
	nameAttr string
}

func (rq *Request) RadioGroup(nba *NamedBoolArray, params ...interface{}) (rl []RadioElement) {
	nameAttr := `name="` + MakeID() + `"`
	nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			rl = append(rl, RadioElement{e: rq.NewElement(NewUiRadio(nb)), nameAttr: nameAttr})
		}
	})
	return
}

// Radio renders a HTML input element of type 'radio'.
func (r RadioElement) Radio(attrs ...string) template.HTML {
	var sb strings.Builder
	r.e.ui.(*UiRadio).JawsRender(r.e, &sb, append(attrs, r.nameAttr))
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
