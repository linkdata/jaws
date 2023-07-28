package jaws

import (
	"html/template"
	"io"
)

type UiNumber struct {
	UiInputFloat
}

func (ui *UiNumber) JawsRender(e *Element, w io.Writer) error {
	return ui.UiInputFloat.WriteHtmlInput(e, w, "number")
}

func NewUiNumber(tags []interface{}, vp ValueProxy) (ui *UiNumber) {
	ui = &UiNumber{
		UiInputFloat: UiInputFloat{
			UiInput: UiInput{
				UiHtml:     UiHtml{Tags: tags},
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Number(tagitem interface{}, val interface{}, attrs ...interface{}) template.HTML {
	return rq.UI(NewUiNumber(ProcessTags(tagitem), MakeValueProxy(val)), attrs...)
}
