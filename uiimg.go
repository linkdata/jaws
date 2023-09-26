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

func (ui *UiImg) SrcAttr(e *Element) string {
	src := ui.JawsGetString(e)
	if len(src) < 1 || src[0] != '"' {
		return strconv.Quote(src)
	}
	return src
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.StringGetter)
	attrs := append(ui.parseParams(e, params), "src="+ui.SrcAttr(e))
	writeUiDebug(e, w)
	maybePanic(WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...))
}

func (ui *UiImg) JawsUpdate(u Updater) {
	u.SetAttr("src", ui.SrcAttr(u.Element))
}

func NewUiImg(g StringGetter) *UiImg {
	return &UiImg{
		StringGetter: g,
	}
}

func (rq *Request) Img(imageSrc interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(makeStringGetter(imageSrc)), params...)
}
