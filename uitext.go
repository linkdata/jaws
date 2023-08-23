package jaws

import (
	"html/template"
	"io"
)

type UiText struct {
	UiInputText
}

func (ui *UiText) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputText.WriteHtmlInput(e, w, "text")
}

func NewUiText(tags []interface{}, vp ValueProxy) (ui *UiText) {
	ui = &UiText{
		UiInputText: UiInputText{
			UiInput: NewUiInput(tags, vp),
		},
	}
	return
}

func (rq *Request) Text(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiText(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
