package jaws

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"strings"
)

// Optionally you may also implement ClickHandler and EventHandler
type UI interface {
	JawsTags(rq *Request) (tags []interface{})
	JawsRender(e *Element, w io.Writer) (err error)
	JawsUpdate(e *Element) (err error)
}

func (rq *Request) UI(ui UI, params ...interface{}) template.HTML {
	tags := ui.JawsTags(rq)
	rq.mu.Lock()
	elem := rq.newElementLocked(tags, ui, params)
	rq.mu.Unlock()
	var b bytes.Buffer
	if err := rq.Jaws.Log(ui.JawsRender(elem, &b)); err != nil {
		b.WriteString(fmt.Sprintf("<!-- jaws.UI(%T).JawsRender(): %s -->", ui, strings.ReplaceAll(err.Error(), "--", "==")))
	}
	return template.HTML(b.String())
}

func (rq *Request) Update(tag interface{}) {
	var todo []*Element
	rq.mu.RLock()
	todo = append(todo, rq.tagMap[tag]...)
	rq.mu.RUnlock()
	var b bytes.Buffer
	for _, elem := range todo {
		if err := elem.UI().JawsRender(elem, &b); err != nil {
			rq.Jaws.MustLog(err)
		}
		b.Reset()
	}
}
