package jaws

import "html/template"

type Templater interface {
	JawsTemplates(rq *Request, tl []Template) []Template
}

type defaultTemplater struct {
	templ  *template.Template
	tagger Tagger
}

func (dt *defaultTemplater) JawsTemplates(rq *Request, tl []Template) []Template {
	for _, dot := range dt.tagger.JawsTags(rq, nil) {
		tl = append(tl, Template{Template: dt.templ, Dot: dot})
	}
	return tl
}

func (rq *Request) Templater(templ interface{}, tagger Tagger) Templater {
	var tp *template.Template
	switch v := templ.(type) {
	case string:
		tp = rq.Jaws.Template.Lookup(v)
	case *template.Template:
		tp = v
	default:
		panic("jaws: Request.Templater(): template must be string or *template.Template")
	}
	return &defaultTemplater{
		templ:  tp,
		tagger: tagger,
	}
}
