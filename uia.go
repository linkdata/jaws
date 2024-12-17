package jaws

import (
	"io"
)

type UiA struct {
	UiHTMLInner
}

func (ui *UiA) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "a", "", params)
}

func NewUiA(innerHTML HTMLGetter) *UiA {
	return &UiA{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) A(innerHTML any, params ...any) error {
	return rq.UI(NewUiA(MakeHTMLGetter(innerHTML)), params...)
}
