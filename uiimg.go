package jaws

import (
	"html/template"
	"io"
	"strconv"
)

type UiImg struct {
	UiHtml
	StringGetter
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []any) error {
	ui.parseGetter(e, ui.StringGetter)
	srcattr := template.HTMLAttr("src=" + strconv.Quote(ui.JawsGetString(e))) // #nosec G203
	attrs := append(e.ParseParams(params), srcattr)
	return WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...)
}

func (ui *UiImg) JawsUpdate(e *Element) {
	e.SetAttr("src", ui.JawsGetString(e))
}

func NewUiImg(g StringGetter) *UiImg {
	return &UiImg{
		StringGetter: g,
	}
}

func (rq RequestWriter) Img(imageSrc any, params ...any) error {
	return rq.UI(NewUiImg(makeStringGetter(imageSrc)), params...)
}
