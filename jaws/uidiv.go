package jaws

import (
	"io"
)

type UiDiv struct {
	UiHTMLInner
}

func (ui *UiDiv) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "div", "", params)
}

func NewUiDiv(innerHTML HTMLGetter) *UiDiv {
	return &UiDiv{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Div(innerHTML any, params ...any) error {
	return rq.UI(NewUiDiv(MakeHTMLGetter(innerHTML)), params...)
}
