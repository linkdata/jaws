package jaws

import (
	"bytes"
	"html/template"
	"io"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement Tagger, ClickHandler and/or EventHandler.
type UI interface {
	JawsRender(e *Element, w io.Writer, params ...interface{})
	JawsUpdate(u Updater)
}

func (rq *Request) UI(ui UI, params ...interface{}) template.HTML {
	elem := rq.NewElement(ui)
	var b bytes.Buffer
	ui.JawsRender(elem, &b, params...)
	if tagger, ok := ui.(Tagger); ok {
		tags := tagger.JawsTags(rq, nil)
		rq.mu.Lock()
		for _, tag := range tags {
			rq.tagMap[tag] = append(rq.tagMap[tag], elem)
		}
		rq.mu.Unlock()
	}
	return template.HTML(b.String())
}
