package jaws

import (
	"io"
)

type UiSpan struct {
	UiHTMLInner
}

func (ui *UiSpan) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "span", "", params)
}

func NewUiSpan(innerHTML HTMLGetter) *UiSpan {
	return &UiSpan{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Span(innerHTML any, params ...any) error {
	return rq.UI(NewUiSpan(MakeHTMLGetter(innerHTML)), params...)
}
