package jaws

import (
	"io"
)

type UiLi struct {
	UiHTMLInner
}

func (ui *UiLi) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "li", "", params)
}

func NewUiLi(innerHTML HTMLGetter) *UiLi {
	return &UiLi{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Li(innerHTML any, params ...any) error {
	return rq.UI(NewUiLi(MakeHTMLGetter(innerHTML)), params...)
}
