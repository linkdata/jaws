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
	var sb strings.Builder
	rq.JawsRender(rq.NewElement(ui), &sb, params)
	return template.HTML(sb.String())
}

func (rq *Request) JawsRender(elem *Element, w io.Writer, params []interface{}) {
	elem.ui.JawsRender(elem, w, params)
	rq.queueMoveToEnd(elem.jid)
}
