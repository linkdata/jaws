package jaws

import "html/template"

type Container interface {
	JawsContains(rq *Request) (tl []Template)
}

type defaultContainer struct {
	templ  *template.Template
	tagger Tagger
}

func (dt *defaultContainer) JawsContains(rq *Request) (tl []Template) {
	for _, dot := range dt.tagger.JawsTags(rq, nil) {
		tl = append(tl, Template{Template: dt.templ, Dot: dot})
	}
	return tl
}

func (rq *Request) MakeContainer(templ interface{}, tagger Tagger) Container {
	var tp *template.Template
	switch v := templ.(type) {
	case string:
		tp = rq.Jaws.Template.Lookup(v)
	case *template.Template:
		tp = v
	default:
		panic("Request.MakeContainer(): template must be string or *template.Template")
	}
	return &defaultContainer{
		templ:  tp,
		tagger: tagger,
	}
}
