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

func (rq *Request) RadioGroup(nba *NamedBoolArray) (rel []RadioElement) {
	nameAttr := `name="` + MakeID() + `"`
	nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			rel = append(rel, RadioElement{
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
func (re RadioElement) Radio(params ...interface{}) template.HTML {
	var sb strings.Builder
	re.radio.Render(&sb, append(params, re.nameAttr))
	return template.HTML(sb.String()) // #nosec G203
}

// Label renders a HTML label element.
func (re *RadioElement) Label(params ...interface{}) template.HTML {
	var sb strings.Builder
	forAttr := string(re.radio.jid.AppendQuote([]byte("for=")))
	re.label.Render(&sb, append(params, forAttr))
	return template.HTML(sb.String()) // #nosec G203
}
