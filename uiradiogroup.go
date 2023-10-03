package jaws

import (
	"html/template"
	"strings"
)

type RadioElement struct {
	radio    *Element
	label    *Element
	nameAttr string
}

func (rq *Request) RadioGroup(nba *NamedBoolArray) (rl []RadioElement) {
	nameAttr := `name="` + MakeID() + `"`
	nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			rl = append(rl, RadioElement{
				radio:    rq.NewElement(NewUiRadio(nb)),
				label:    rq.NewElement(NewUiLabel(nb)),
				nameAttr: nameAttr,
			},
			)
		}
	})
	return
}

// Radio renders a HTML input element of type 'radio'.
func (r RadioElement) Radio(params ...interface{}) template.HTML {
	var sb strings.Builder
	r.radio.Render(&sb, append(params, r.nameAttr))
	return template.HTML(sb.String())
}

// Label renders a HTML label element.
func (r *RadioElement) Label(params ...interface{}) template.HTML {
	var sb strings.Builder
	forAttr := string(r.radio.jid.AppendQuote([]byte("for=")))
	r.label.Render(&sb, append(params, forAttr))
	return template.HTML(sb.String())
}
