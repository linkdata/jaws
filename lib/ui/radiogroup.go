package ui

import (
	"html/template"
	"strings"

	"github.com/linkdata/jaws"
	"github.com/linkdata/jaws/lib/named"
)

// RadioElement contains the input and label elements for one radio option.
type RadioElement struct {
	radio    *jaws.Element
	label    *jaws.Element
	nameAttr string
}

// Radio renders an HTML input element of type radio.
func (re RadioElement) Radio(params ...any) template.HTML {
	var sb strings.Builder
	re.radio.Jaws.MustLog(re.radio.JawsRender(&sb, append(params, re.nameAttr)))
	return template.HTML(sb.String()) // #nosec G203
}

// Label renders an HTML label element.
func (re RadioElement) Label(params ...any) template.HTML {
	var sb strings.Builder
	forAttr := string(re.radio.Jid().AppendQuote([]byte("for=")))
	re.label.Jaws.MustLog(re.label.JawsRender(&sb, append(params, forAttr)))
	return template.HTML(sb.String()) // #nosec G203
}

// RadioGroup creates radio and label elements for each value in nba.
func (rw RequestWriter) RadioGroup(nba *named.BoolArray) (rel []RadioElement) {
	nameAttr := `name="` + jaws.MakeID() + `"`
	nba.ReadLocked(func(nbl []*named.Bool) {
		for _, nb := range nbl {
			rel = append(rel, RadioElement{
				radio:    rw.Request.NewElement(NewRadio(nb)),
				label:    rw.Request.NewElement(NewLabel(nb)),
				nameAttr: nameAttr,
			},
			)
		}
	})
	return
}
