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

func NewUiLabel(vp Getter) (ui *UiLabel) {
	ui = &UiLabel{
		UiHtmlInner{
			UiGetter{
				Getter: vp,
			},
		},
	}
	return
}

func (rq *Request) Label(innerHtml interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiLabel(MakeValueProxy(innerHtml)), params...)
}
