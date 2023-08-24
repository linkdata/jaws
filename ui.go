package jaws

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"io"
)

type UI interface {
	JawsCreate(self UI, rq *Request, data []interface{}) (elem *Element, err error)
	JawsRender(e *Element, w io.Writer) (err error)
	JawsUpdate(e *Element) (err error)
	EventHandler
}

func (rq *Request) UI(ui UI, data ...interface{}) template.HTML {
	elem, err := ui.JawsCreate(ui, rq, data)
	if err != nil {
		rq.Jaws.MustLog(err)
		return template.HTML(fmt.Sprintf("<!-- jaws.UI(%T).JawsCreate(): %s -->", ui, html.EscapeString(err.Error())))
	}
	var b bytes.Buffer
	if err := ui.JawsRender(elem, &b); err != nil {
		rq.Jaws.MustLog(err)
		b.WriteString(fmt.Sprintf("<!-- jaws.UI(%T).JawsRender(): %s -->", ui, html.EscapeString(err.Error())))
	}
	return template.HTML(b.String())
}

func (rq *Request) Update(tags []interface{}) {
	var todo []*Element
	rq.mu.RLock()
	for _, tag := range tags {
		todo = append(todo, rq.tagMap[tag]...)
	}
	rq.mu.RUnlock()
	var b bytes.Buffer
	for _, elem := range todo {
		if err := elem.UI().JawsRender(elem, &b); err != nil {
			rq.Jaws.MustLog(err)
		}
		b.Reset()
	}
}

type Ui interface {
	JawsUi(rq *Request, attrs ...string) template.HTML
}

func (rq *Request) Ui(elem Ui, attrs ...string) template.HTML {
	return elem.JawsUi(rq, attrs...)
}
