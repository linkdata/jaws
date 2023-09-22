package jaws

import (
	"html/template"
	"io"
)

type UiLabel struct {
	UiHtmlInner
}

func (ui *UiLabel) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.UiHtmlInner.WriteHtmlInner(e, w, "label", "", params...)
}

func NewUiLabel(vp ValueProxy) (ui *UiLabel) {
	ui = &UiLabel{
		UiHtmlInner{
			UiValueProxy{
				ValueProxy: vp,
			},
		},
	}
	return
}

func (rq *Request) Label(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLabel(MakeValueProxy(innerHtml)), params...)
}
