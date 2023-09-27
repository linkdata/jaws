package jaws

import (
	"html/template"
	"io"
	"strings"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement ClickHandler and/or EventHandler.
type UI interface {
	JawsRender(e *Element, w io.Writer, params []interface{})
	JawsUpdate(e *Element)
}

func (rq *Request) UI(ui UI, params ...interface{}) template.HTML {
	elem := rq.NewElement(ui)
	var sb strings.Builder
	ui.JawsRender(elem, &sb, params)
	return template.HTML(sb.String())
}
