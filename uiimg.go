package jaws

import (
	"html/template"
	"io"
	"strconv"
)

type UiImg struct {
	UiHtml
	StringSetter
}

func (ui *UiImg) SrcAttr(e *Element) string {
	src := ui.JawsGetString(e)
	if len(src) < 1 || src[0] != '"' {
		return strconv.Quote(src)
	}
	return src
}

func (ui *UiImg) JawsRender(e *Element, w io.Writer, params []interface{}) {
	ui.parseGetter(e, ui.StringSetter)
	attrs := append(ui.parseParams(e, params), "src="+ui.SrcAttr(e))
	maybePanic(WriteHtmlInner(w, e.Jid(), "img", "", "", attrs...))
}

func (ui *UiImg) JawsUpdate(u *Element) {
	u.SetAttr("src", ui.SrcAttr(u))
}

func NewUiImg(g StringSetter) *UiImg {
	return &UiImg{
		StringSetter: g,
	}
}

func (rq *Request) Img(imageSrc interface{}, params ...interface{}) template.HTML {
	return rq.UI(NewUiImg(makeStringSetter(imageSrc)), params...)
}
