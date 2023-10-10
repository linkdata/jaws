package jaws

import (
	"fmt"
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
	return template.HTML(sb.String()) // #nosec G203
}

func (rq *Request) JawsRender(elem *Element, w io.Writer, params []interface{}) {
	elem.ui.JawsRender(elem, w, params)
	if rq.Jaws.Debug {
		var sb strings.Builder
		_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", elem.jid, elem.ui)
		for i, tag := range elem.Request.TagsOf(elem) {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(TagString(tag))
		}
		sb.WriteByte(']')
		_, _ = w.Write([]byte(strings.ReplaceAll(sb.String(), "-->", "==>") + " -->"))
	}
}
