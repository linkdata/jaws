package jaws

import (
	"html/template"

	"github.com/linkdata/jaws/what"
)

type Radio struct {
	nba *NamedBoolArray
	rq  *Request
	fn  InputTextFn
	NamedBool
}

// Jid returns the JaWS ID.
func (r *Radio) Jid() string {
	return r.nba.JidOf(r.Name)
}

// Radio renders a HTML input element of type 'radio'.
func (r *Radio) Radio(attrs ...string) template.HTML {
	jid := r.Jid()
	attrs = append(attrs, `name="`+r.nba.Jid+`"`)
	attrs = append(attrs, `id="`+jid+`"`)
	if r.Checked {
		attrs = append(attrs, `checked`)
	}
	r.rq.RegisterEventFn(jid, func(rq *Request, wht what.What, id, val string) error {
		return r.nba.radioEventFn(rq, wht, id, val, r.fn)
	})
	return HtmlInput(jid, "radio", "", attrs...)
}

// Label renders a HTML label element.
func (r *Radio) Label(attrs ...string) template.HTML {
	jid := r.nba.JidOf(r.Name)
	attrs = append(attrs, `for="`+jid+`"`)
	return HtmlInner("", "label", "", r.Html, attrs...)
}
