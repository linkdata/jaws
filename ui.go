package jaws

import (
	"fmt"
	"io"
	"strings"
)

// If any of these functions panic, the Request will be closed and the panic logged.
// Optionally you may also implement ClickHandler and/or EventHandler.
type UI interface {
	// JawsRender is called once per Element when rendering the initial webpage.
	// Do not call this yourself unless it's from within another JawsRender implementation.
	JawsRender(e *Element, w io.Writer, params []any) error

	// JawsUpdate is called for an Element that has been marked dirty to update it's HTML.
	// Do not call this yourself unless it's from within another JawsUpdate implementation.
	JawsUpdate(e *Element)
}

func (rq *Request) JawsRender(elem *Element, w io.Writer, params []any) (err error) {
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
