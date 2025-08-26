package jaws

import (
	"io"
)

type UiTr struct {
	UiHTMLInner
}

func (ui *UiTr) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "tr", "", params)
}

func NewUiTr(innerHTML HTMLGetter) *UiTr {
	return &UiTr{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Tr(innerHTML any, params ...any) error {
	return rq.UI(NewUiTr(MakeHTMLGetter(innerHTML)), params...)
}
