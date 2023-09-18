package jaws

import "html/template"

type Container interface {
	// JawsContains must return a slice of UI objects. The slice contents must not be modified after returning it.
	JawsContains(rq *Request) (contents []UI)
}

type templateContainer struct {
	templ  *template.Template
	tagger Tagger
}

func (dt *templateContainer) JawsContains(rq *Request) (contents []UI) {
	for _, dot := range dt.tagger.JawsTags(rq, nil) {
		contents = append(contents, Template{Template: dt.templ, Dot: dot})
	}
	return contents
}

func (rq *Request) MakeContainer(templ interface{}, tagger Tagger) Container {
	return &templateContainer{
		templ:  rq.MustTemplate(templ),
		tagger: tagger,
	}
}
