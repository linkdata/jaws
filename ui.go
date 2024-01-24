package jaws

import (
	"fmt"
	"io"
	"strings"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement ClickHandler and/or EventHandler.
type UI interface {
	JawsRender(e *Element, w io.Writer, params []interface{}) error
	JawsUpdate(e *Element)
}

func (rq *Request) JawsRender(elem *Element, w io.Writer, params []interface{}) (err error) {
	if err = elem.Ui().JawsRender(elem, w, params); err == nil {
		if rq.Jaws.Debug {
			var sb strings.Builder
			_, _ = fmt.Fprintf(&sb, "<!-- id=%q %T tags=[", elem.Jid(), elem.Ui())
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
	return
}
