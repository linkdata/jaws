package jaws

import (
	"io"
)

type UiLabel struct {
	UiHTMLInner
}

func (ui *UiLabel) JawsRender(e *Element, w io.Writer, params []any) error {
	return ui.renderInner(e, w, "label", "", params)
}

func NewUiLabel(innerHTML HTMLGetter) *UiLabel {
	return &UiLabel{
		UiHTMLInner{
			HTMLGetter: innerHTML,
		},
	}
}

func (rq RequestWriter) Label(innerHTML any, params ...any) error {
	return rq.UI(NewUiLabel(MakeHTMLGetter(innerHTML)), params...)
}
