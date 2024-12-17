package jaws

import (
	"io"
)

type UiButton struct {
	UiHTMLInner
}

func (ui *UiButton) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "button", "button", params)
}

func NewUiButton(innerHTML HTMLGetter) *UiButton {
	return &UiButton{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Button(innerHTML any, params ...any) error {
	return rq.UI(NewUiButton(MakeHTMLGetter(innerHTML)), params...)
}
