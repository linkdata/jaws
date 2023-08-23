package jaws

import (
	"html/template"
	"io"
)

type UiCheckbox struct {
	UiInputBool
}

func (ui *UiCheckbox) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputBool.WriteHtmlInput(e, w, "checkbox")
}

func NewUiCheckbox(tags []interface{}, vp ValueProxy) (ui *UiCheckbox) {
	ui = &UiCheckbox{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(tags, vp),
		},
	}
	return
}

func (rq *Request) Checkbox(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiCheckbox(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
