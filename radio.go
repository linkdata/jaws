package jaws

import "html/template"

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
	r.rq.RegisterEventFn(jid, func(rq *Request, id, evt, val string) error {
		return r.nba.radioEventFn(rq, id, evt, val, r.fn)
	})
	return HtmlInput(jid, "radio", "", attrs...)
}

// Label renders a HTML label element.
func (r *Radio) Label(attrs ...string) template.HTML {
	jid := r.nba.JidOf(r.Name)
	b := make([]byte, 0)
	b = append(b, `<label for="`...)
	b = append(b, []byte(jid)...)
	b = append(b, '"')
	for _, attr := range attrs {
		b = append(b, ' ')
		b = append(b, attr...)
	}
	b = append(b, '>')
	b = append(b, []byte(r.Html)...)
	b = append(b, `</label>`...)
	return template.HTML(b)
}
