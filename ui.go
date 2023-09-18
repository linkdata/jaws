package jaws

import (
	"bytes"
	"html/template"
	"io"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement Tagger, ClickHandler and/or EventHandler.
type UI interface {
	JawsRender(e *Element, w io.Writer)
	JawsUpdate(u Updater)
}

func (rq *Request) UI(ui UI, params ...interface{}) template.HTML {
	elem := rq.NewElement(ui, params)
	var b bytes.Buffer
	ui.JawsRender(elem, &b)
	return template.HTML(b.String())
}
