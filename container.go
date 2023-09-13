package jaws

import "html/template"

type Container interface {
	JawsContains(rq *Request) (l []Template)
}

type defaultContainer struct {
	templ  *template.Template
	tagger Tagger
}

func (dt *defaultContainer) JawsContains(rq *Request) (l []Template) {
	for _, dot := range dt.tagger.JawsTags(rq, nil) {
		l = append(l, Template{Template: dt.templ, Dot: dot})
	}
	return l
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
