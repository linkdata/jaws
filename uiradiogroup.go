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

func (rw RequestWriter) RadioGroup(nba *NamedBoolArray) (rel []RadioElement) {
	nameAttr := `name="` + MakeID() + `"`
	nba.ReadLocked(func(nbl []*NamedBool) {
		for _, nb := range nbl {
			rel = append(rel, RadioElement{
				radio:    rw.Request().NewElement(NewUiRadio(nb)),
				label:    rw.Request().NewElement(NewUiLabel(nb)),
				nameAttr: nameAttr,
			},
			)
		}
	})
	return
}

// Radio renders a HTML input element of type 'radio'.
func (re RadioElement) Radio(params ...any) template.HTML {
	var sb strings.Builder
	maybePanic(re.radio.JawsRender(&sb, append(params, re.nameAttr)))
	return template.HTML(sb.String()) // #nosec G203
}

// Label renders a HTML label element.
func (re RadioElement) Label(params ...any) template.HTML {
	var sb strings.Builder
	forAttr := string(re.radio.Jid().AppendQuote([]byte("for=")))
	maybePanic(re.label.JawsRender(&sb, append(params, forAttr)))
	return template.HTML(sb.String()) // #nosec G203
}
