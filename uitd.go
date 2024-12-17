package jaws

import (
	"io"
)

type UiTd struct {
	UiHTMLInner
}

func (ui *UiTd) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "td", "", params)
}

func NewUiTd(innerHTML HTMLGetter) *UiTd {
	return &UiTd{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Td(innerHTML any, params ...any) error {
	return rq.UI(NewUiTd(MakeHTMLGetter(innerHTML)), params...)
}
