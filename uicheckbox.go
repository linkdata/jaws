package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiInputBool.WriteHtmlInput(e, w, "checkbox", params...)
}

func MakeUiCheckbox(g BoolGetter) UiCheckbox {
	return UiCheckbox{
		UiInputBool{
			BoolGetter: g,
		},
	}
}

func (rq *Request) Checkbox(value interface{}, params ...interface{}) template.HTML {
	ui := MakeUiCheckbox(makeBoolGetter(value))
	return rq.UI(&ui, params...)
}
