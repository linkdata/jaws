package jaws

import (
	"bytes"
	"html/template"
	"io"
)

// Optionally you may also implement Tagger, ClickHandler and/or EventHandler.
// If any of these panics, the Request will be closed and the panic logged.
type UI interface {
	JawsRender(e *Element, w io.Writer)
	JawsUpdate(e *Element, u Updater)
}

func (rq *Request) UI(ui UI, params ...interface{}) template.HTML {
	elem := rq.NewElement(ui, params)
	var b bytes.Buffer
	ui.JawsRender(elem, &b)
	return template.HTML(b.String())
}
