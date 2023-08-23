package jaws

import (
	"html/template"
	"io"
)

type UiRadio struct {
	UiInputBool
}

func (ui *UiRadio) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputBool.WriteHtmlInput(e, w, "radio")
}

func NewUiRadio(tags []interface{}, vp ValueProxy) (ui *UiRadio) {
	ui = &UiRadio{
		UiInputBool: UiInputBool{
			UiInput: NewUiInput(tags, vp),
		},
	}
	return
}

func (rq *Request) Radio(tagitem interface{}, val interface{}, data ...interface{}) template.HTML {
	return rq.UI(NewUiRadio(ProcessTags(tagitem), MakeValueProxy(val)), data...)
}
